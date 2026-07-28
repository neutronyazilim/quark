package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCommitConflictUninstall(t *testing.T) {
	root := t.TempDir()
	db, err := OpenDB(root)
	if err != nil {
		t.Fatal(err)
	}
	installer := NewInstaller(root, db)

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "content")
	if err := os.WriteFile(srcFile, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []StagedFile{
		{RelPath: "usr/bin", IsDir: true, Mode: 0o755},
		{RelPath: "usr/bin/hello", SrcPath: srcFile, Mode: 0o755},
	}
	pkgA := &Package{Name: "pkg-a", Version: "1.0", Format: FormatNative, Source: SourceNative}

	if err := installer.CheckConflicts(pkgA.Name, files); err != nil {
		t.Fatalf("unexpected conflict on fresh install: %v", err)
	}
	if err := installer.Commit(pkgA, files); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	installedPath := filepath.Join(root, "usr/bin/hello")
	data, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("installed file content = %q, want hello", data)
	}

	got, err := db.Get("pkg-a")
	if err != nil || got == nil {
		t.Fatalf("db.Get after commit: pkg=%v err=%v", got, err)
	}
	if len(got.Files) != 2 {
		t.Errorf("recorded %d files, want 2", len(got.Files))
	}

	pkgB := &Package{Name: "pkg-b", Version: "1.0", Format: FormatNative, Source: SourceNative}
	err = installer.CheckConflicts(pkgB.Name, files)
	var conflictErr *ErrConflict
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}
	if !asConflict(err, &conflictErr) {
		t.Fatalf("expected *ErrConflict, got %T: %v", err, err)
	}
	if conflictErr.Owner != "pkg-a" {
		t.Errorf("conflict owner = %q, want pkg-a", conflictErr.Owner)
	}

	if err := installer.CheckConflicts(pkgA.Name, files); err != nil {
		t.Errorf("self-reinstall should not conflict: %v", err)
	}

	if err := installer.Uninstall("pkg-a"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(installedPath); !os.IsNotExist(err) {
		t.Errorf("file still exists after uninstall: %v", err)
	}
	if got, _ := db.Get("pkg-a"); got != nil {
		t.Errorf("db record still present after uninstall")
	}
}

func asConflict(err error, target **ErrConflict) bool {
	if e, ok := err.(*ErrConflict); ok {
		*target = e
		return true
	}
	return false
}
