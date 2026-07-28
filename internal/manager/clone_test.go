package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/clone"
	"github.com/neutronyazilim/quark/internal/core"
)

func newCloneTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	rootDir := t.TempDir()
	return newTestManager(t, rootDir), rootDir
}

func TestExportThenImportRoundTrip(t *testing.T) {
	srcManager, srcRoot := newCloneTestManager(t)

	qpkPath := buildMockQpk(t, t.TempDir(), "hello")
	if err := srcManager.Install(qpkPath); err != nil {
		t.Fatalf("Install: %v", err)
	}

	homeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("export FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(t.TempDir(), "myprofile.qprof")
	if err := srcManager.Export(destPath, []string{".zshrc"}, homeDir); err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf(".qprof was not written: %v", err)
	}

	dstManager, _ := newCloneTestManager(t)
	dstHome := t.TempDir()

	summary, err := dstManager.Import(destPath, dstHome, false)
	if err == nil {
		t.Fatal("expected Import to report an error for the foreign 'hello' package, got nil")
	}
	if summary == nil {
		t.Fatal("expected a non-nil summary even on partial failure")
	}
	if len(summary.PackageErrors) != 1 {
		t.Errorf("PackageErrors = %v, want exactly 1 (foreign 'hello')", summary.PackageErrors)
	}
	if summary.DotfilesRestored != 1 {
		t.Errorf("DotfilesRestored = %d, want 1", summary.DotfilesRestored)
	}
	data, err := os.ReadFile(filepath.Join(dstHome, ".zshrc"))
	if err != nil || string(data) != "export FOO=bar\n" {
		t.Errorf("restored .zshrc = %q, err=%v", data, err)
	}

	if _, err := os.Stat(filepath.Join(srcRoot, "usr/bin/hello")); err != nil {
		t.Fatalf("source install did not actually happen: %v", err)
	}
}

func TestImportReinstallsNativePackage(t *testing.T) {
	dstManager, dstRoot := newCloneTestManager(t)

	artifactDir := t.TempDir()
	qpkPath := buildMockQpk(t, artifactDir, "hello")

	mock := &mockSource{tag: "native", pkg: map[string]core.RemotePackage{
		"hello": {
			Name: "hello", Version: "1.0", Format: core.FormatNative, Source: core.SourceNative,
			URL: "mock://hello",
		},
	}}
	dstManager.sourceByTag["native"] = mock
	dstManager.sources = append(dstManager.sources, mock)

	cacheFile := filepath.Join(dstManager.cfg.CacheDir, string(core.SourceNative), "packages", "hello")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(qpkPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	manifestPkgs := []*core.Package{{Name: "hello", Version: "1.0", Source: core.SourceNative, Format: core.FormatNative}}
	manifest := filepath.Join(t.TempDir(), "profile.qprof")
	if err := clone.WriteProfile(manifest, clone.BuildManifest(manifestPkgs, nil, "seed-host"), t.TempDir()); err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	summary, err := dstManager.Import(manifest, t.TempDir(), false)
	if err != nil {
		t.Fatalf("Import: %v (summary=%+v)", err, summary)
	}
	if summary.PackagesInstalled != 1 {
		t.Fatalf("PackagesInstalled = %d, want 1 (errors=%v)", summary.PackagesInstalled, summary.PackageErrors)
	}
	if _, err := os.Stat(filepath.Join(dstRoot, "usr/bin/hello")); err != nil {
		t.Errorf("hello was not actually installed on the destination root: %v", err)
	}
}

func TestImportSkipsAlreadyInstalledSameVersion(t *testing.T) {
	m, _ := newCloneTestManager(t)
	qpkPath := buildMockQpk(t, t.TempDir(), "hello")
	if err := m.Install(qpkPath); err != nil {
		t.Fatalf("Install: %v", err)
	}

	manifest := filepath.Join(t.TempDir(), "profile.qprof")
	if err := m.Export(manifest, nil, t.TempDir()); err != nil {
		t.Fatalf("Export: %v", err)
	}

	summary, err := m.Import(manifest, t.TempDir(), false)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if summary.PackagesSkipped != 1 || summary.PackagesInstalled != 0 || len(summary.PackageErrors) != 0 {
		t.Errorf("summary = %+v, want 1 skipped, 0 installed, 0 errors", summary)
	}
}
