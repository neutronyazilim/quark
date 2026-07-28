package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeArEntry(buf *bytes.Buffer, name string, data []byte) {
	header := make([]byte, 60)
	copy(header, fmt.Sprintf("%-16s", name))
	copy(header[16:], fmt.Sprintf("%-12d", 0))
	copy(header[28:], fmt.Sprintf("%-6d", 0))
	copy(header[34:], fmt.Sprintf("%-6d", 0))
	copy(header[40:], fmt.Sprintf("%-8s", "644"))
	copy(header[48:], fmt.Sprintf("%-10d", len(data)))
	copy(header[58:], "`\n")
	buf.Write(header)
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte('\n')
	}
}

func buildGzipTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	gw.Write(tarBuf.Bytes())
	gw.Close()
	return gzBuf.Bytes()
}

func buildFakeDeb(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")

	writeArEntry(&buf, "debian-binary", []byte("2.0\n"))

	control := "Package: hello-deb\nVersion: 1.2.3\nDepends: libc6 (>= 2.34), libfoo\nDescription: a test package\n"
	writeArEntry(&buf, "control.tar.gz", buildGzipTar(t, map[string]string{"./control": control}))

	writeArEntry(&buf, "data.tar.gz", buildGzipTar(t, map[string]string{
		"./usr/bin/hello-deb": "#!/bin/sh\necho hi\n",
	}))

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFakeDeb(t *testing.T) {
	debPath := filepath.Join(t.TempDir(), "hello-deb_1.2.3_amd64.deb")
	buildFakeDeb(t, debPath)

	adapter := New(t.TempDir())
	pkg, files, err := adapter.Install(debPath)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if pkg.Name != "hello-deb" || pkg.Version != "1.2.3" {
		t.Errorf("pkg = %+v, want name=hello-deb version=1.2.3", pkg)
	}
	want := []string{"libc6", "libfoo"}
	if len(pkg.Depends) != len(want) {
		t.Fatalf("Depends = %v, want %v", pkg.Depends, want)
	}

	var found bool
	for _, f := range files {
		if f.RelPath == "usr/bin/hello-deb" {
			found = true
			data, err := os.ReadFile(f.SrcPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "#!/bin/sh\necho hi\n" {
				t.Errorf("staged content mismatch: %q", data)
			}
		}
	}
	if !found {
		t.Errorf("usr/bin/hello-deb not staged, got %+v", files)
	}
}
