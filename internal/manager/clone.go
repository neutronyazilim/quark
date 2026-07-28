package manager

import (
	"fmt"
	"os"

	"github.com/neutronyazilim/quark/internal/clone"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/ui"
)

func (m *Manager) Export(destPath string, dotfiles []string, homeDir string) error {
	pkgs, err := m.db.List()
	if err != nil {
		return fmt.Errorf("kurulu paket listesi alınamadı: %w", err)
	}
	host, _ := os.Hostname()
	manifest := clone.BuildManifest(pkgs, dotfiles, host)
	if err := clone.WriteProfile(destPath, manifest, homeDir); err != nil {
		return err
	}
	ui.Ok("%s yazıldı: %d paket, %d dotfile", destPath, len(manifest.Packages), len(manifest.Dotfiles))
	return nil
}

type ImportSummary struct {
	PackagesInstalled int
	PackagesSkipped   int
	PackageErrors     []string
	DotfilesRestored  int
	DotfilesSkipped   int
	DotfileErrors     []string
}

func (s *ImportSummary) failed() bool {
	return len(s.PackageErrors) > 0 || len(s.DotfileErrors) > 0
}

func (m *Manager) Import(srcPath string, homeDir string, force bool) (*ImportSummary, error) {
	manifest, stageDir, err := clone.ReadProfile(srcPath, m.stagingRoot)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stageDir)

	summary := &ImportSummary{}
	for _, entry := range manifest.Packages {
		if existing, _ := m.db.Get(entry.Name); existing != nil && existing.Version == entry.Version {
			summary.PackagesSkipped++
			continue
		}

		spec, err := importSpec(entry)
		if err != nil {
			summary.PackageErrors = append(summary.PackageErrors, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}

		ui.Info("kuruluyor: %s %s", entry.Name, entry.Version)
		if err := m.Install(spec); err != nil {
			summary.PackageErrors = append(summary.PackageErrors, fmt.Sprintf("%s: %v", entry.Name, err))
			continue
		}
		summary.PackagesInstalled++
	}

	for _, r := range clone.RestoreDotfiles(manifest, stageDir, homeDir, force) {
		switch {
		case r.Err != nil:
			summary.DotfileErrors = append(summary.DotfileErrors, fmt.Sprintf("%s: %v", r.Path, r.Err))
		case r.Skipped:
			summary.DotfilesSkipped++
		default:
			summary.DotfilesRestored++
		}
	}

	if summary.failed() {
		return summary, fmt.Errorf("içe aktarma kısmen başarısız oldu (%d paket, %d dotfile hatası) - ayrıntılar için özete bakın",
			len(summary.PackageErrors), len(summary.DotfileErrors))
	}
	return summary, nil
}

func importSpec(entry clone.PackageEntry) (string, error) {
	switch core.Source(entry.Source) {
	case core.SourceNative:
		return entry.Name, nil
	case core.SourceArch:
		return "arch:" + entry.Name, nil
	case core.SourceDebian:
		return "deb:" + entry.Name, nil
	default:
		return "", fmt.Errorf("kaynağı yok (%q) - muhtemelen yerel bir dosyadan kurulmuştu, yeniden kurulamıyor", entry.Source)
	}
}
