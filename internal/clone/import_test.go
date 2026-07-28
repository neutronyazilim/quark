package clone

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreDotfilesWithoutForceSkipsExisting(t *testing.T) {
	stageDir := t.TempDir()
	homeDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(stageDir, ".zshrc"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &Manifest{Dotfiles: []string{".zshrc"}}
	results := RestoreDotfiles(manifest, stageDir, homeDir, false)

	if len(results) != 1 || !results[0].Skipped || results[0].Err != nil {
		t.Fatalf("results = %+v, want one Skipped entry", results)
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	if err != nil || string(data) != "old content" {
		t.Errorf(".zshrc = %q, err=%v, want untouched old content", data, err)
	}
}

func TestRestoreDotfilesWithForceOverwrites(t *testing.T) {
	stageDir := t.TempDir()
	homeDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(stageDir, ".zshrc"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &Manifest{Dotfiles: []string{".zshrc"}}
	results := RestoreDotfiles(manifest, stageDir, homeDir, true)

	if len(results) != 1 || results[0].Skipped || results[0].Err != nil {
		t.Fatalf("results = %+v, want one restored (non-skipped) entry", results)
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	if err != nil || string(data) != "new content" {
		t.Errorf(".zshrc = %q, err=%v, want overwritten new content", data, err)
	}
}

func TestRestoreDotfilesNestedDirFreshDestination(t *testing.T) {
	stageDir := t.TempDir()
	homeDir := t.TempDir()

	nvimDir := filepath.Join(stageDir, ".config", "nvim")
	if err := os.MkdirAll(nvimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- init\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &Manifest{Dotfiles: []string{".config/nvim"}}
	results := RestoreDotfiles(manifest, stageDir, homeDir, false)

	if len(results) != 1 || results[0].Err != nil || results[0].Skipped {
		t.Fatalf("results = %+v, want one restored entry", results)
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".config", "nvim", "init.lua"))
	if err != nil || string(data) != "-- init\n" {
		t.Errorf("restored init.lua = %q, err=%v", data, err)
	}
}

func TestReadProfileRejectsFileWithoutManifest(t *testing.T) {

	srcPath := filepath.Join(t.TempDir(), "not-a-profile.qprof")
	if err := os.WriteFile(srcPath, []byte("plain text, not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stageDir, err := ReadProfile(srcPath, t.TempDir())
	if err == nil {
		t.Fatal("expected an error reading a non-gzip file, got nil")
	}
	if stageDir != "" {
		if _, statErr := os.Stat(stageDir); statErr == nil {
			t.Errorf("stageDir %s was left behind after a failed ReadProfile", stageDir)
		}
	}
}
