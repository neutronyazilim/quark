package generic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func buildTarGz(t *testing.T, files map[string]string) string {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()

	path := filepath.Join(t.TempDir(), "pkg.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	gw.Write(tarBuf.Bytes())
	gw.Close()
	return path
}

func TestInstallRelocatableTarball(t *testing.T) {
	path := buildTarGz(t, map[string]string{"bin/mytool": "#!/bin/sh\necho hi\n"})
	a := New(t.TempDir(), "usr/local")
	_, files, err := a.Install(path)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, f := range files {
		if !f.IsDir {
			got = f.RelPath
		}
	}
	want := "usr/local/bin/mytool"
	if got != want {
		t.Errorf("relocatable tarball installed at %q, want %q (should be prefixed, not dumped into real /bin)", got, want)
	}
}

func TestInstallFHSRootedTarball(t *testing.T) {
	path := buildTarGz(t, map[string]string{"usr/bin/mytool": "#!/bin/sh\necho hi\n"})
	a := New(t.TempDir(), "usr/local")
	_, files, err := a.Install(path)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, f := range files {
		if !f.IsDir {
			got = f.RelPath
		}
	}
	want := "usr/bin/mytool"
	if got != want {
		t.Errorf("FHS-rooted tarball installed at %q, want %q (should be used as-is, not double-prefixed)", got, want)
	}
}
