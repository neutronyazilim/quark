package pkgformat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildInstallRoundTrip(t *testing.T) {
	srcRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "usr/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(srcRoot, "usr/bin/hello")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	qpkPath := filepath.Join(t.TempDir(), "hello-1.0.qpk")
	meta := Metadata{Name: "hello", Version: "1.0", Description: "test package", Depends: []string{"libc"}}
	if err := Build(srcRoot, meta, qpkPath); err != nil {
		t.Fatalf("Build: %v", err)
	}

	stagingRoot := t.TempDir()
	adapter := New(stagingRoot)
	if !adapter.Detect(qpkPath) {
		t.Fatalf("Detect() = false for %s", qpkPath)
	}

	pkg, files, err := adapter.Install(qpkPath)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if pkg.Name != "hello" || pkg.Version != "1.0" {
		t.Errorf("pkg = %+v, want name=hello version=1.0", pkg)
	}
	if len(pkg.Depends) != 1 || pkg.Depends[0] != "libc" {
		t.Errorf("pkg.Depends = %v, want [libc]", pkg.Depends)
	}

	var foundBin bool
	for _, f := range files {
		if f.RelPath == "usr/bin/hello" {
			foundBin = true
			data, err := os.ReadFile(f.SrcPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "#!/bin/sh\necho hi\n" {
				t.Errorf("staged file content mismatch: %q", data)
			}
		}
	}
	if !foundBin {
		t.Errorf("usr/bin/hello not found in staged files: %+v", files)
	}
}
