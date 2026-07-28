package arch

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/neutronyazilim/quark/internal/core"
)

type Adapter struct {
	StagingRoot string
}

func New(stagingRoot string) *Adapter { return &Adapter{StagingRoot: stagingRoot} }

func (a *Adapter) Format() core.Format { return core.FormatArch }

func (a *Adapter) Detect(path string) bool {
	return strings.HasSuffix(path, ".pkg.tar.zst")
}

var depNameRe = regexp.MustCompile(`^[^<>=]+`)

func depName(raw string) string {
	m := depNameRe.FindString(raw)
	return strings.TrimSpace(m)
}

func (a *Adapter) Install(path string) (*core.Package, []core.StagedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("pkg.tar.zst açılamadı %s: %w - dosya bozuk olabilir, yeniden indirin", path, err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	var pkg *core.Package
	var files []core.StagedFile
	stageDir := filepath.Join(a.StagingRoot, fmt.Sprintf("arch-%d", os.Getpid()))
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return nil, nil, err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("pkg.tar.zst okunamadı %s: %w", path, err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")

		switch {
		case name == ".PKGINFO":
			var buf strings.Builder
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, nil, err
			}
			pkg = parsePkgInfo(buf.String())

		case name == ".MTREE" || name == ".BUILDINFO" || name == ".INSTALL" || name == ".CHANGELOG" || strings.HasPrefix(name, "."):

			continue

		default:
			if name == "" {
				continue
			}
			stagedPath := filepath.Join(stageDir, filepath.FromSlash(name))

			switch hdr.Typeflag {
			case tar.TypeDir:
				files = append(files, core.StagedFile{RelPath: name, IsDir: true, Mode: os.FileMode(hdr.Mode)})
			case tar.TypeSymlink:
				files = append(files, core.StagedFile{RelPath: name, IsLink: true, LinkTo: hdr.Linkname})
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
					return nil, nil, err
				}
				out, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
				if err != nil {
					return nil, nil, err
				}
				if _, err := io.Copy(out, tr); err != nil {
					out.Close()
					return nil, nil, err
				}
				out.Close()
				files = append(files, core.StagedFile{RelPath: name, SrcPath: stagedPath, Mode: os.FileMode(hdr.Mode)})
			}
		}
	}

	if pkg == nil {
		return nil, nil, fmt.Errorf("%s .PKGINFO içermiyor - geçerli bir Arch paketi değil", path)
	}
	pkg.Format = core.FormatArch
	pkg.Source = core.SourceArch
	return pkg, files, nil
}

func parsePkgInfo(text string) *core.Package {
	pkg := &core.Package{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "pkgname":
			pkg.Name = val
		case "pkgver":
			pkg.Version = val
		case "pkgdesc":
			pkg.Description = val
		case "depend":
			pkg.Depends = append(pkg.Depends, depName(val))
		}
	}
	return pkg
}
