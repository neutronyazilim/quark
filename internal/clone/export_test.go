package clone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/core"
)

func TestBuildManifestSortsDotfilesAndMapsFields(t *testing.T) {
	pkgs := []*core.Package{
		{Name: "htop", Version: "3.3.0", Source: core.SourceArch, Format: core.FormatArch},
		{Name: "neutron-editor", Version: "1.2.0", Source: core.SourceNative, Format: core.FormatNative},
	}
	m := BuildManifest(pkgs, []string{".zshrc", ".config/nvim"}, "test-host")

	if m.Version != ManifestVersion {
		t.Errorf("Version = %d, want %d", m.Version, ManifestVersion)
	}
	if m.Host != "test-host" {
		t.Errorf("Host = %q, want test-host", m.Host)
	}
	if len(m.Packages) != 2 {
		t.Fatalf("Packages = %+v, want 2 entries", m.Packages)
	}
	if m.Packages[0].Name != "htop" || m.Packages[0].Source != "arch" || m.Packages[0].Format != "pkg.tar.zst" {
		t.Errorf("Packages[0] = %+v, want htop/arch/pkg.tar.zst", m.Packages[0])
	}
	wantDotfiles := []string{".config/nvim", ".zshrc"}
	if len(m.Dotfiles) != 2 || m.Dotfiles[0] != wantDotfiles[0] || m.Dotfiles[1] != wantDotfiles[1] {
		t.Errorf("Dotfiles = %v, want %v (sorted)", m.Dotfiles, wantDotfiles)
	}
}

func TestWriteReadProfileRoundTrip(t *testing.T) {
	homeDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("export FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	if err := os.MkdirAll(nvimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".config/nvim", filepath.Join(homeDir, "nvim-link")); err != nil {
		t.Fatal(err)
	}

	pkgs := []*core.Package{
		{Name: "htop", Version: "3.3.0", Source: core.SourceArch, Format: core.FormatArch},
	}
	manifest := BuildManifest(pkgs, []string{".zshrc", ".config/nvim", "nvim-link"}, "test-host")

	destPath := filepath.Join(t.TempDir(), "myprofile.qprof")
	if err := WriteProfile(destPath, manifest, homeDir); err != nil {
		t.Fatalf("WriteProfile: %v", err)
	}

	got, stageDir, err := ReadProfile(destPath, t.TempDir())
	if err != nil {
		t.Fatalf("ReadProfile: %v", err)
	}
	defer os.RemoveAll(stageDir)

	if got.Host != "test-host" || len(got.Packages) != 1 || got.Packages[0].Name != "htop" {
		t.Errorf("read-back manifest = %+v, want host test-host + [htop]", got)
	}

	data, err := os.ReadFile(filepath.Join(stageDir, ".zshrc"))
	if err != nil || string(data) != "export FOO=bar\n" {
		t.Errorf(".zshrc content = %q, err=%v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(stageDir, ".config", "nvim", "init.lua"))
	if err != nil || string(data) != "-- init\n" {
		t.Errorf("init.lua content = %q, err=%v", data, err)
	}
	link, err := os.Readlink(filepath.Join(stageDir, "nvim-link"))
	if err != nil || link != ".config/nvim" {
		t.Errorf("nvim-link = %q, err=%v, want .config/nvim", link, err)
	}
}

func TestWriteProfileMissingDotfileFailsLoudly(t *testing.T) {
	homeDir := t.TempDir()
	manifest := BuildManifest(nil, []string{".does-not-exist"}, "")
	destPath := filepath.Join(t.TempDir(), "p.qprof")

	err := WriteProfile(destPath, manifest, homeDir)
	if err == nil {
		t.Fatal("expected an error for a missing dotfile, got nil")
	}
}
