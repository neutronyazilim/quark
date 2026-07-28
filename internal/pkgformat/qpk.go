package pkgformat

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/neutronyazilim/quark/internal/core"
)

const MetadataFile = "metadata.json"
const DataPrefix = "data/"

type Metadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Depends     []string `json:"depends,omitempty"`
}

type Adapter struct {
	StagingRoot string
}

func New(stagingRoot string) *Adapter { return &Adapter{StagingRoot: stagingRoot} }

func (a *Adapter) Format() core.Format { return core.FormatNative }

func (a *Adapter) Detect(path string) bool {
	return strings.HasSuffix(path, ".qpk")
}

func (a *Adapter) Install(path string) (*core.Package, []core.StagedFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf(".qpk açılamadı %s: %w - dosya bozuk olabilir", path, err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	var meta *Metadata
	var files []core.StagedFile
	stageDir := filepath.Join(a.StagingRoot, fmt.Sprintf("qpk-%d", os.Getpid()))
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return nil, nil, err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf(".qpk okunamadı %s: %w", path, err)
		}

		switch {
		case hdr.Name == MetadataFile:
			var buf strings.Builder
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, nil, err
			}
			meta = &Metadata{}
			if err := json.Unmarshal([]byte(buf.String()), meta); err != nil {
				return nil, nil, fmt.Errorf("metadata.json ayrıştırılamadı: %w", err)
			}

		case strings.HasPrefix(hdr.Name, DataPrefix):
			relPath := strings.TrimPrefix(hdr.Name, DataPrefix)
			if relPath == "" {
				continue
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
				out, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
				if err != nil {
					return nil, nil, err
				}
				if _, err := io.Copy(out, tr); err != nil {
					out.Close()
					return nil, nil, err
				}
				out.Close()
				files = append(files, core.StagedFile{RelPath: relPath, SrcPath: stagedPath, Mode: os.FileMode(hdr.Mode)})
			}
		}
	}

	if meta == nil {
		return nil, nil, fmt.Errorf(".qpk metadata.json içermiyor: %s - bu geçerli bir quark paketi değil", path)
	}

	pkg := &core.Package{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Format:      core.FormatNative,
		Depends:     meta.Depends,
	}
	return pkg, files, nil
}

func Build(srcRoot string, meta Metadata, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw, err := zstd.NewWriter(f)
	if err != nil {
		return err
	}
	defer zw.Close()

	tw := tar.NewWriter(zw)
	defer tw.Close()

	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: MetadataFile, Size: int64(len(metaJSON)), Mode: 0o644}); err != nil {
		return err
	}
	if _, err := tw.Write(metaJSON); err != nil {
		return err
	}

	return filepath.Walk(srcRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		tarName := DataPrefix + filepath.ToSlash(rel)

		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			hdr := &tar.Header{Name: tarName, Typeflag: tar.TypeSymlink, Linkname: link, Mode: int64(info.Mode().Perm())}
			return tw.WriteHeader(hdr)
		}
		if info.IsDir() {
			hdr := &tar.Header{Name: tarName + "/", Typeflag: tar.TypeDir, Mode: int64(info.Mode().Perm())}
			return tw.WriteHeader(hdr)
		}

		hdr := &tar.Header{Name: tarName, Typeflag: tar.TypeReg, Size: info.Size(), Mode: int64(info.Mode().Perm())}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		file, err := os.Open(p)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
}

func Base(p string) string { return path.Base(p) }
