package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/manager"
	"github.com/neutronyazilim/quark/internal/ui"
)

const usage = `quark - NeutronLinux paket yöneticisi

Kullanım:
  quark :i <paket|arch:paket|deb:paket|dosya>   paket kur
  quark :r <paket>                              paket kaldır
  quark :s <sorgu>                               paket ara
  quark :u                                       kaynak metadata'sını senkronize et
  quark :up                                      kurulu paketleri yükselt
  quark :info <paket>                            paket detayı
  quark :list                                    kurulu paketleri listele

  quark :export <profil>[.qprof] [dotfile ...]   sistemi bir .qprof'a klonla
  quark :import <profil>.qprof                   bir .qprof'u bu sisteme uygula

Seçenekler:
  --config <yol>   config dosyası (varsayılan: /etc/quark/quark.conf)
  --force          glibc ABI uyumluluk kontrolünü atla ('quark :i' ile,
                    riski göze alarak - normalde uyumsuz bir paket hiç
                    diske yazılmadan reddedilir); 'quark :import' ile ise
                    var olan dotfile'ların üzerine yazılmasına izin ver
                    (varsayılan: var olan dosyalar atlanır, dokunulmaz)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	configPath := config.DefaultPath
	args := os.Args[1:]
	verb := args[0]
	rest := args[1:]

	if verb == "-h" || verb == "--help" || verb == "help" {
		fmt.Print(usage)
		return
	}

	force := false
	var filtered []string
	for i := 0; i < len(rest); i++ {
		switch {
		case rest[i] == "--config" && i+1 < len(rest):
			configPath = rest[i+1]
			i++
		case rest[i] == "--force":
			force = true
		default:
			filtered = append(filtered, rest[i])
		}
	}
	rest = filtered

	cfg, err := config.Load(configPath)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	m, err := manager.New(cfg)
	if err != nil {
		ui.Error("başlatılamadı: %v", err)
		os.Exit(1)
	}
	m.Force = force

	var runErr error
	switch verb {
	case ":i", ":install":
		runErr = cmdInstall(m, rest)
	case ":r", ":remove":
		runErr = cmdRemove(m, rest)
	case ":s", ":search":
		runErr = cmdSearch(m, rest)
	case ":u", ":sync":
		runErr = m.Sync()
	case ":up", ":upgrade":
		runErr = m.Upgrade()
	case ":info":
		runErr = cmdInfo(m, rest)
	case ":list":
		runErr = cmdList(m)
	case ":export":
		runErr = cmdExport(m, rest)
	case ":import":
		runErr = cmdImport(m, rest, force)
	default:
		fmt.Print(usage)
		os.Exit(1)
	}

	if runErr != nil {
		ui.Error("%v", runErr)
		os.Exit(1)
	}
}

func cmdInstall(m *manager.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :i <paket|arch:paket|deb:paket|dosya>")
	}
	for _, spec := range args {
		if err := m.Install(spec); err != nil {
			return err
		}
	}
	return nil
}

func cmdRemove(m *manager.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :r <paket>")
	}
	for _, name := range args {
		if err := m.Remove(name); err != nil {
			return err
		}
	}
	return nil
}

func cmdSearch(m *manager.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :s <sorgu>")
	}
	results, err := m.Search(args[0])
	if err != nil {
		return err
	}
	if len(results) == 0 {
		ui.Info("sonuç yok")
		return nil
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	for _, r := range results {
		fmt.Printf("%s %s %s\n", ui.Tag(string(r.Source)), r.Name, r.Version)
		if r.Description != "" {
			ui.Muted("%s", r.Description)
		}
	}
	return nil
}

func cmdInfo(m *manager.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :info <paket>")
	}
	name := args[0]
	installed, remote, err := m.Info(name)
	if err != nil {
		return err
	}
	if installed != nil {
		fmt.Printf("%s %s %s\n", ui.Tag(string(installed.Source)), installed.Name, installed.Version)
		fmt.Printf("format:    %s\n", installed.Format)
		fmt.Printf("kurulu:    evet (%s)\n", installed.InstalledAt.Format(time.RFC3339))
		if installed.Description != "" {
			fmt.Printf("açıklama:  %s\n", installed.Description)
		}
		if len(installed.Depends) > 0 {
			fmt.Printf("bağımlı:   %v\n", installed.Depends)
		}
		fmt.Printf("dosyalar:  %d\n", len(installed.Files))
		return nil
	}
	if remote != nil {
		fmt.Printf("%s %s %s\n", ui.Tag(string(remote.Source)), remote.Name, remote.Version)
		fmt.Printf("format:    %s\n", remote.Format)
		fmt.Printf("kurulu:    hayır\n")
		if remote.Description != "" {
			fmt.Printf("açıklama:  %s\n", remote.Description)
		}
		if len(remote.Depends) > 0 {
			fmt.Printf("bağımlı:   %v\n", remote.Depends)
		}
		return nil
	}
	return fmt.Errorf("%s bulunamadı", name)
}

func cmdList(m *manager.Manager) error {
	pkgs, err := m.List()
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		ui.Info("kurulu paket yok")
		return nil
	}
	for _, p := range pkgs {
		fmt.Printf("%s %s %s (%s)\n", ui.Tag(string(p.Source)), p.Name, p.Version, p.Format)
	}
	return nil
}

func cmdExport(m *manager.Manager, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :export <profil-dosyası>[.qprof] [dotfile-yolu ...]")
	}
	destPath := args[0]
	if !strings.HasSuffix(destPath, ".qprof") {
		destPath += ".qprof"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("ev dizini bulunamadı: %w", err)
	}
	dotfiles, err := normalizeDotfiles(args[1:], homeDir)
	if err != nil {
		return err
	}
	return m.Export(destPath, dotfiles, homeDir)
}

func normalizeDotfiles(paths []string, homeDir string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel := p
		if filepath.IsAbs(p) {
			r, err := filepath.Rel(homeDir, p)
			if err != nil || strings.HasPrefix(r, "..") {
				return nil, fmt.Errorf("%s ev dizini (%s) dışında - sadece ev dizini altındaki dosyalar dışa aktarılabilir", p, homeDir)
			}
			rel = r
		}
		out = append(out, filepath.ToSlash(filepath.Clean(rel)))
	}
	return out, nil
}

func cmdImport(m *manager.Manager, args []string, force bool) error {
	if len(args) == 0 {
		return fmt.Errorf("kullanım: quark :import <profil-dosyası>.qprof")
	}
	srcPath := args[0]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("ev dizini bulunamadı: %w", err)
	}

	summary, importErr := m.Import(srcPath, homeDir, force)
	if summary == nil {

		return importErr
	}

	fmt.Println()
	ui.Ok("%d paket kuruldu, %d paket zaten güncel", summary.PackagesInstalled, summary.PackagesSkipped)
	ui.Ok("%d dotfile geri yüklendi, %d dotfile atlandı (zaten vardı, --force ile üzerine yazılır)",
		summary.DotfilesRestored, summary.DotfilesSkipped)
	if len(summary.PackageErrors) > 0 {
		ui.Error("kurulamayan paketler:")
		for _, e := range summary.PackageErrors {
			ui.Muted("%s", e)
		}
	}
	if len(summary.DotfileErrors) > 0 {
		ui.Error("geri yüklenemeyen dotfile'lar:")
		for _, e := range summary.DotfileErrors {
			ui.Muted("%s", e)
		}
	}
	return importErr
}
