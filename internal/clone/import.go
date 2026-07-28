package clone

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ReadProfile(srcPath, stagingRoot string) (manifest *Manifest, stageDir string, err error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, "", fmt.Errorf("profil açılamadı %s: %w", srcPath, err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", fmt.Errorf("profil açılamadı %s: %w - .qprof dosyası bozuk olabilir", srcPath, err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, "", err
	}
	stageDir, err = os.MkdirTemp(stagingRoot, "qprof-")
	if err != nil {
		return nil, "", err
	}

	defer func() {
		if err != nil {
			os.RemoveAll(stageDir)
		}
	}()

	for {
		hdr, terr := tr.Next()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return nil, "", fmt.Errorf("profil okunamadı %s: %w", srcPath, terr)
		}

		switch {
		case hdr.Name == ManifestFile:
			data, rerr := io.ReadAll(tr)
			if rerr != nil {
				return nil, "", rerr
			}
			manifest = &Manifest{}
			if jerr := json.Unmarshal(data, manifest); jerr != nil {
				return nil, "", fmt.Errorf("manifest.json ayrıştırılamadı: %w", jerr)
			}

		case strings.HasPrefix(hdr.Name, DotfilesPrefix):
			rel := strings.TrimPrefix(hdr.Name, DotfilesPrefix)
			if rel == "" {
				continue
			}
			dest := filepath.Join(stageDir, filepath.FromSlash(rel))

			switch hdr.Typeflag {
			case tar.TypeDir:
				if merr := os.MkdirAll(dest, os.FileMode(hdr.Mode)); merr != nil {
					return nil, "", merr
				}
			case tar.TypeSymlink:
				if merr := os.MkdirAll(filepath.Dir(dest), 0o755); merr != nil {
					return nil, "", merr
				}
				if serr := os.Symlink(hdr.Linkname, dest); serr != nil {
					return nil, "", serr
				}
			case tar.TypeReg:
				if merr := os.MkdirAll(filepath.Dir(dest), 0o755); merr != nil {
					return nil, "", merr
				}
				out, oerr := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
				if oerr != nil {
					return nil, "", oerr
				}
				if _, cerr := io.Copy(out, tr); cerr != nil {
					out.Close()
					return nil, "", cerr
				}
				out.Close()
			}
		}
	}

	if manifest == nil {
		return nil, "", fmt.Errorf("%s bir manifest.json içermiyor - bu geçerli bir .qprof dosyası değil", srcPath)
	}
	return manifest, stageDir, nil
}

type DotfileResult struct {
	Path    string
	Skipped bool
	Err     error
}

func RestoreDotfiles(manifest *Manifest, stageDir, homeDir string, force bool) []DotfileResult {
	results := make([]DotfileResult, 0, len(manifest.Dotfiles))
	for _, rel := range manifest.Dotfiles {
		src := filepath.Join(stageDir, filepath.FromSlash(rel))
		dst := filepath.Join(homeDir, filepath.FromSlash(rel))

		if !force {
			if _, err := os.Lstat(dst); err == nil {
				results = append(results, DotfileResult{Path: rel, Skipped: true})
				continue
			}
		}

		if err := os.RemoveAll(dst); err != nil {
			results = append(results, DotfileResult{Path: rel, Err: fmt.Errorf("eski dosya kaldırılamadı: %w", err)})
			continue
		}
		if err := copyTree(src, dst); err != nil {
			results = append(results, DotfileResult{Path: rel, Err: err})
			continue
		}
		results = append(results, DotfileResult{Path: rel})
	}
	return results
}

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)

	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil

	default:
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	}
}
