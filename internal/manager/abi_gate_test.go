package manager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/config"
)

func findHostFile(t *testing.T, candidates ...string) string {
	t.Helper()
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("required host file not found, skipping ABI-gate test")
	return ""
}

func buildTarGzOf(t *testing.T, destPath, relPath, srcFile string) {
	t.Helper()
	data, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	if err := tw.WriteHeader(&tar.Header{Name: relPath, Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	tw.Close()

	f, err := os.Create(destPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	gw.Write(tarBuf.Bytes())
	gw.Close()
}

func newTestManager(t *testing.T, rootDir string) *Manager {
	t.Helper()
	cfg := config.Default()
	cfg.RootDir = rootDir
	cfg.CacheDir = t.TempDir()
	cfg.Sources.Native.Enabled = false
	cfg.Sources.Arch.Enabled = false
	cfg.Sources.Debian.Enabled = false
	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestABIGate_BlocksThenForceOverrides(t *testing.T) {

	targetLibc := findHostFile(t, "/mnt/data/heryarrak/neutronlinux/sysroot/usr/lib/libc.so.6")
	newLib := findHostFile(t, "/usr/lib/libncursesw.so.6", "/lib/x86_64-linux-gnu/libncursesw.so.6")

	buildPkg := func(t *testing.T) string {
		rootDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(rootDir, "usr/lib"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := copyFileForTest(targetLibc, filepath.Join(rootDir, "usr/lib/libc.so.6")); err != nil {
			t.Fatal(err)
		}
		return rootDir
	}

	pkgPath := filepath.Join(t.TempDir(), "toonew.tar.gz")
	buildTarGzOf(t, pkgPath, "usr/lib/libncursesw.so.6", newLib)

	t.Run("blocked without force", func(t *testing.T) {
		rootDir := buildPkg(t)
		m := newTestManager(t, rootDir)

		err := m.Install(pkgPath)
		if err == nil {
			t.Fatal("expected install to be rejected for glibc ABI mismatch, got nil error")
		}
		if _, statErr := os.Stat(filepath.Join(rootDir, "usr/lib/libncursesw.so.6")); !os.IsNotExist(statErr) {
			t.Errorf("incompatible file should never have been written to disk, stat err = %v", statErr)
		}
	})

	t.Run("allowed with force", func(t *testing.T) {
		rootDir := buildPkg(t)
		m := newTestManager(t, rootDir)
		m.Force = true

		if err := m.Install(pkgPath); err != nil {
			t.Fatalf("expected --force to allow the install through, got: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(rootDir, "usr/lib/libncursesw.so.6")); statErr != nil {
			t.Errorf("--force should have written the file, stat err = %v", statErr)
		}
	})
}

func copyFileForTest(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
