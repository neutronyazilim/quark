package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/pkgformat"
)

func TestInstallLocalQpkAndRemove(t *testing.T) {
	rootDir := t.TempDir()
	cacheDir := t.TempDir()

	cfg := config.Default()
	cfg.RootDir = rootDir
	cfg.CacheDir = cacheDir
	cfg.Sources.Native.Enabled = false
	cfg.Sources.Arch.Enabled = false
	cfg.Sources.Debian.Enabled = false

	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	srcRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "usr/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "usr/bin/hello"), []byte("hi"), 0o755); err != nil {
		t.Fatal(err)
	}
	qpkPath := filepath.Join(t.TempDir(), "hello-1.0.qpk")
	if err := pkgformat.Build(srcRoot, pkgformat.Metadata{Name: "hello", Version: "1.0"}, qpkPath); err != nil {
		t.Fatal(err)
	}

	if err := m.Install(qpkPath); err != nil {
		t.Fatalf("Install: %v", err)
	}

	installedFile := filepath.Join(rootDir, "usr/bin/hello")
	if data, err := os.ReadFile(installedFile); err != nil || string(data) != "hi" {
		t.Fatalf("installed file = %q, err=%v", data, err)
	}

	pkgs, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].Name != "hello" {
		t.Fatalf("List() = %+v, want [hello]", pkgs)
	}

	if err := m.Remove("hello"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(installedFile); !os.IsNotExist(err) {
		t.Errorf("file still present after Remove: %v", err)
	}
}
