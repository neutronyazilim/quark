package clone

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/neutronyazilim/quark/internal/core"
)

const ManifestFile = "manifest.json"

const DotfilesPrefix = "dotfiles/"

func BuildManifest(pkgs []*core.Package, dotfiles []string, host string) *Manifest {
	entries := make([]PackageEntry, len(pkgs))
	for i, p := range pkgs {
		entries[i] = PackageEntry{
			Name:    p.Name,
			Version: p.Version,
			Source:  string(p.Source),
			Format:  string(p.Format),
		}
	}
	sorted := append([]string(nil), dotfiles...)
	sort.Strings(sorted)
	return &Manifest{
		Version:   ManifestVersion,
		CreatedAt: time.Now().UTC(),
		Host:      host,
		Packages:  entries,
		Dotfiles:  sorted,
	}
}

func WriteProfile(destPath string, manifest *Manifest, homeDir string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("profil dosyası oluşturulamadı %s: %w", destPath, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	metaJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: ManifestFile, Size: int64(len(metaJSON)), Mode: 0o644}); err != nil {
		return err
	}
	if _, err := tw.Write(metaJSON); err != nil {
		return err
	}

	for _, rel := range manifest.Dotfiles {
		src := filepath.Join(homeDir, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		if err != nil {
			return fmt.Errorf("dotfile bulunamadı %s: %w", src, err)
		}
		if err := addToTar(tw, src, DotfilesPrefix+rel, info); err != nil {
			return fmt.Errorf("dotfile arşivlenemedi %s: %w", src, err)
		}
	}
	return nil
}

func addToTar(tw *tar.Writer, path, tarName string, info os.FileInfo) error {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{
			Name: tarName, Typeflag: tar.TypeSymlink, Linkname: link, Mode: int64(info.Mode().Perm()),
		})

	case info.IsDir():
		if err := tw.WriteHeader(&tar.Header{Name: tarName + "/", Typeflag: tar.TypeDir, Mode: int64(info.Mode().Perm())}); err != nil {
			return err
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			childInfo, err := e.Info()
			if err != nil {
				return err
			}
			if err := addToTar(tw, filepath.Join(path, e.Name()), tarName+"/"+e.Name(), childInfo); err != nil {
				return err
			}
		}
		return nil

	default:
		hdr := &tar.Header{Name: tarName, Typeflag: tar.TypeReg, Size: info.Size(), Mode: int64(info.Mode().Perm())}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	}
}
