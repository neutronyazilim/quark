package generic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/ulikunitz/xz"
)

type Adapter struct {
	StagingRoot string

	Prefix string
}

func New(stagingRoot, prefix string) *Adapter {
	if prefix == "" {
		prefix = "usr/local"
	}
	return &Adapter{StagingRoot: stagingRoot, Prefix: prefix}
}

func (a *Adapter) Format() core.Format { return core.FormatGeneric }

func (a *Adapter) Detect(path string) bool {
	return strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz") ||
		strings.HasSuffix(path, ".tar.xz") || strings.HasSuffix(path, ".tar.zst")
}

var fhsRoots = map[string]bool{
	"usr": true, "etc": true, "opt": true, "var": true,
}

func (a *Adapter) Install(path string) (*core.Package, []core.StagedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	r, err := openCompressed(path, f)
	if err != nil {
		return nil, nil, fmt.Errorf("arşiv açılamadı %s: %w", path, err)
	}
	tr := tar.NewReader(r)

	var headers []*tar.Header
	var bodies [][]byte
	fhsRooted := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("arşiv okunamadı %s: %w", path, err)
		}
		top := strings.SplitN(strings.TrimPrefix(hdr.Name, "./"), "/", 2)[0]
		if fhsRoots[top] {
			fhsRooted = true
		}
		var body bytes.Buffer
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.Copy(&body, tr); err != nil {
				return nil, nil, err
			}
		}
		headers = append(headers, hdr)
		bodies = append(bodies, body.Bytes())
	}

	stageDir := filepath.Join(a.StagingRoot, fmt.Sprintf("generic-%d", os.Getpid()))
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return nil, nil, err
	}

	var files []core.StagedFile
	base := filepath.Base(path)
	name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".gz"), ".xz"), ".zst")
	name = strings.TrimSuffix(name, ".tar")
	name = strings.TrimSuffix(name, ".tgz")

	for i, hdr := range headers {
		relPath := strings.TrimPrefix(hdr.Name, "./")
		if relPath == "" || relPath == "." {
			continue
		}
		if !fhsRooted {
			relPath = filepath.ToSlash(filepath.Join(a.Prefix, relPath))
		}
		stagedPath := filepath.Join(stageDir, filepath.FromSlash(relPath))

		switch hdr.Typeflag {
		case tar.TypeDir:
			files = append(files, core.StagedFile{RelPath: relPath, IsDir: true, Mode: os.FileMode(hdr.Mode)})
		case tar.TypeSymlink:
			files = append(files, core.StagedFile{RelPath: relPath, IsLink: true, LinkTo: hdr.Linkname})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(stagedPath, bodies[i], os.FileMode(hdr.Mode)); err != nil {
				return nil, nil, err
			}
			files = append(files, core.StagedFile{RelPath: relPath, SrcPath: stagedPath, Mode: os.FileMode(hdr.Mode)})
		}
	}

	pkg := &core.Package{
		Name:    name,
		Version: "unknown",
		Format:  core.FormatGeneric,
		Source:  core.SourceForeign,
	}
	return pkg, files, nil
}

func openCompressed(path string, f *os.File) (io.Reader, error) {
	switch {
	case strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz"):
		return gzip.NewReader(f)
	case strings.HasSuffix(path, ".tar.xz"):
		return xz.NewReader(f)
	case strings.HasSuffix(path, ".tar.zst"):
		zr, err := zstd.NewReader(f)
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	}
	return nil, fmt.Errorf("bilinmeyen arşiv türü: %s", path)
}
