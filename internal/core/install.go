package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type StagedFile struct {
	RelPath string

	SrcPath string
	Mode    os.FileMode
	IsDir   bool
	IsLink  bool
	LinkTo  string
}

type Installer struct {
	Root string
	DB   *DB
}

func NewInstaller(root string, db *DB) *Installer {
	return &Installer{Root: root, DB: db}
}

type ErrConflict struct {
	Path  string
	Owner string
}

func (e *ErrConflict) Error() string {
	return fmt.Sprintf("dosya çakışması: %s zaten %q paketine ait", e.Path, e.Owner)
}

func (in *Installer) CheckConflicts(pkgName string, files []StagedFile) error {
	for _, f := range files {
		if f.IsDir {
			continue
		}
		owner, err := in.DB.OwnerOf("/" + f.RelPath)
		if err != nil {
			return err
		}
		if owner != "" && owner != pkgName {
			return &ErrConflict{Path: "/" + f.RelPath, Owner: owner}
		}
	}
	return nil
}

func (in *Installer) Commit(pkg *Package, files []StagedFile) error {
	var installedPaths []string
	for _, f := range files {
		dst := filepath.Join(in.Root, f.RelPath)

		if f.IsDir {
			if err := os.MkdirAll(dst, f.Mode); err != nil {
				return fmt.Errorf("dizin oluşturulamadı %s: %w", dst, err)
			}
			installedPaths = append(installedPaths, "/"+f.RelPath)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("üst dizin oluşturulamadı %s: %w", dst, err)
		}

		if f.IsLink {
			os.Remove(dst)
			if err := os.Symlink(f.LinkTo, dst); err != nil {
				return fmt.Errorf("sembolik bağ oluşturulamadı %s: %w", dst, err)
			}
			installedPaths = append(installedPaths, "/"+f.RelPath)
			continue
		}

		staging := dst + ".quarknew"
		if err := copyFile(f.SrcPath, staging, f.Mode); err != nil {
			return fmt.Errorf("dosya hazırlanamadı %s: %w", dst, err)
		}
		if err := os.Rename(staging, dst); err != nil {
			os.Remove(staging)
			return fmt.Errorf("dosya yerleştirilemedi %s: %w", dst, err)
		}
		installedPaths = append(installedPaths, "/"+f.RelPath)
	}

	pkg.Files = installedPaths
	pkg.InstalledAt = time.Now().UTC()
	return in.DB.Save(pkg)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func (in *Installer) Uninstall(name string) error {
	pkg, err := in.DB.Get(name)
	if err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("%s kurulu değil", name)
	}

	dirs := map[string]bool{}
	for i := len(pkg.Files) - 1; i >= 0; i-- {
		full := filepath.Join(in.Root, pkg.Files[i])
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		if info.IsDir() {
			dirs[full] = true
			continue
		}
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("silinemedi %s: %w", full, err)
		}
	}
	for dir := range dirs {
		os.Remove(dir)
	}
	return in.DB.Remove(name)
}
