package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutronyazilim/quark/internal/abi"
	archadapter "github.com/neutronyazilim/quark/internal/adapters/arch"
	"github.com/neutronyazilim/quark/internal/adapters/debian"
	"github.com/neutronyazilim/quark/internal/adapters/generic"
	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/fetch"
	"github.com/neutronyazilim/quark/internal/pkgformat"
	archsrc "github.com/neutronyazilim/quark/internal/sources/arch"
	debiansrc "github.com/neutronyazilim/quark/internal/sources/debian"
	nativesrc "github.com/neutronyazilim/quark/internal/sources/native"
	"github.com/neutronyazilim/quark/internal/ui"
)

type Manager struct {
	cfg       *config.Config
	db        *core.DB
	installer *core.Installer

	sources     []core.SourceProvider
	sourceByTag map[string]core.SourceProvider

	adapters map[core.Format]core.FormatAdapter

	stagingRoot string

	Force bool
}

func New(cfg *config.Config) (*Manager, error) {
	db, err := core.OpenDB(cfg.RootDir)
	if err != nil {
		return nil, err
	}

	stagingRoot := filepath.Join(cfg.CacheDir, "staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, err
	}

	m := &Manager{
		cfg:         cfg,
		db:          db,
		installer:   core.NewInstaller(cfg.RootDir, db),
		sourceByTag: map[string]core.SourceProvider{},
		adapters:    map[core.Format]core.FormatAdapter{},
		stagingRoot: stagingRoot,
	}

	if cfg.Sources.Native.Enabled {
		s := nativesrc.New(cfg.Sources.Native, cfg.CacheDir)
		m.sources = append(m.sources, s)
		m.sourceByTag["native"] = s
	}
	if cfg.Sources.Arch.Enabled {
		s := archsrc.New(cfg.Sources.Arch, cfg.CacheDir)
		m.sources = append(m.sources, s)
		m.sourceByTag["arch"] = s
	}
	if cfg.Sources.Debian.Enabled {
		s := debiansrc.New(cfg.Sources.Debian, cfg.CacheDir)
		m.sources = append(m.sources, s)
		m.sourceByTag["deb"] = s
		m.sourceByTag["debian"] = s
	}

	nativeAdapter := pkgformat.New(stagingRoot)
	archAd := archadapter.New(stagingRoot)
	debAd := debian.New(stagingRoot)
	genAd := generic.New(stagingRoot, "usr/local")
	m.adapters[core.FormatNative] = nativeAdapter
	m.adapters[core.FormatArch] = archAd
	m.adapters[core.FormatDebian] = debAd
	m.adapters[core.FormatGeneric] = genAd

	return m, nil
}

func (m *Manager) Sync() error {
	if len(m.sources) == 0 {
		ui.Warn("aktif kaynak yok - /etc/quark/quark.conf içinde [sources] bölümünü kontrol edin")
		return nil
	}
	var firstErr error
	for _, src := range m.sources {
		ui.Info("%s senkronize ediliyor...", src.Name())
		if err := src.Sync(); err != nil {
			ui.Error("%s senkronize edilemedi: %v", src.Name(), err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ui.Ok("%s güncel", src.Name())
	}
	return firstErr
}

func isLocalFile(spec string) bool {
	if strings.Contains(spec, "/") || strings.HasSuffix(spec, ".qpk") ||
		strings.HasSuffix(spec, ".pkg.tar.zst") || strings.HasSuffix(spec, ".deb") ||
		strings.HasSuffix(spec, ".tar.gz") || strings.HasSuffix(spec, ".tgz") ||
		strings.HasSuffix(spec, ".tar.xz") || strings.HasSuffix(spec, ".tar.zst") {
		if _, err := os.Stat(spec); err == nil {
			return true
		}
	}
	return false
}

func (m *Manager) Install(spec string) error {
	if isLocalFile(spec) {
		return m.installFile(spec, nil)
	}

	tag, name, pinned := strings.Cut(spec, ":")
	if !pinned {
		name = tag
		tag = ""
	}

	rp, err := m.resolve(tag, name)
	if err != nil {
		return err
	}
	if rp == nil {
		if tag != "" {
			return fmt.Errorf("%s bulunamadı (kaynak: %s) - `quark :u` ile kaynakları güncelleyin", name, tag)
		}
		return fmt.Errorf("%s hiçbir aktif kaynakta bulunamadı - `quark :s %s` ile arayın ya da `quark :u` ile kaynakları güncelleyin", name, name)
	}

	return m.installRemote(rp, map[string]bool{})
}

func (m *Manager) resolve(tag, name string) (*core.RemotePackage, error) {
	if tag != "" {
		src, ok := m.sourceByTag[tag]
		if !ok {
			return nil, fmt.Errorf("bilinmeyen kaynak etiketi %q (bekleniyor: arch, deb)", tag)
		}
		return src.Find(name)
	}
	for _, src := range m.sources {
		rp, err := src.Find(name)
		if err != nil {
			ui.Warn("%s içinde arama başarısız: %v", src.Name(), err)
			continue
		}
		if rp != nil {
			return rp, nil
		}
	}
	return nil, nil
}

func (m *Manager) installRemote(rp *core.RemotePackage, seen map[string]bool) error {
	return m.installRemoteAtDepth(rp, seen, 0)
}

func (m *Manager) installRemoteAtDepth(rp *core.RemotePackage, seen map[string]bool, depth int) error {
	if seen[rp.Name] {
		return nil
	}
	seen[rp.Name] = true

	if existing, _ := m.db.Get(rp.Name); existing != nil && existing.Version == rp.Version {
		ui.Muted("%s %s zaten kurulu", rp.Name, rp.Version)
		return nil
	}

	const maxNonNativeDepDepth = 1
	if rp.Source == core.SourceNative || depth < maxNonNativeDepDepth {
		if err := m.resolveDeps(rp, seen, depth); err != nil {
			return err
		}
	} else if len(rp.Depends) > 0 {
		ui.Warn("%s bağımlılıkları atlandı (derinlik sınırı %d aşıldı - muhtemelen zaten sistemde mevcutlar): %v",
			rp.Name, maxNonNativeDepDepth, rp.Depends)
	}

	packageCacheDir := filepath.Join(m.cfg.CacheDir, string(rp.Source), "packages")
	destPath := filepath.Join(packageCacheDir, filepath.Base(rp.URL))
	label := fmt.Sprintf("%s %s %s", rp.Name, rp.Version, ui.Tag(string(rp.Source)))
	if err := fetch.Download(rp.URL, destPath, label, rp.SHA256); err != nil {
		return err
	}

	return m.installFile(destPath, rp)
}

func (m *Manager) resolveDeps(rp *core.RemotePackage, seen map[string]bool, depth int) error {
	for _, depName := range rp.Depends {
		if existing, _ := m.db.Get(depName); existing != nil {
			continue
		}
		var depRP *core.RemotePackage
		var err error
		if rp.Source == core.SourceNative {
			depRP, err = m.resolve("", depName)
		} else {
			depRP, err = m.resolve(string(sourceTag(rp.Source)), depName)
		}
		if err != nil || depRP == nil {
			if rp.Source == core.SourceNative {
				return fmt.Errorf("bağımlılık çözülemedi: %s (paket %s tarafından isteniyor)", depName, rp.Name)
			}
			ui.Warn("bağımlılık bulunamadı (best-effort atlanıyor): %s (paket %s tarafından isteniyor)", depName, rp.Name)
			continue
		}
		ui.Info("bağımlılık kuruluyor: %s -> %s", rp.Name, depName)
		if err := m.installRemoteAtDepth(depRP, seen, depth+1); err != nil {
			if rp.Source == core.SourceNative {
				return err
			}
			ui.Warn("bağımlılık kurulamadı (best-effort atlanıyor): %s: %v", depName, err)
		}
	}
	return nil
}

func sourceTag(s core.Source) string {
	switch s {
	case core.SourceArch:
		return "arch"
	case core.SourceDebian:
		return "deb"
	default:
		return "native"
	}
}

func (m *Manager) installFile(path string, rp *core.RemotePackage) error {
	var adapter core.FormatAdapter
	if rp != nil {
		adapter = m.adapters[rp.Format]
	} else {
		for _, a := range m.adapters {
			if a.Detect(path) {
				adapter = a
				break
			}
		}
	}
	if adapter == nil {
		return fmt.Errorf("%s için uygun bir format algılanamadı (.qpk, .pkg.tar.zst, .deb, .tar.gz destekleniyor)", path)
	}

	ui.Info("kuruluyor: %s", pkgformat.Base(path))
	pkg, files, err := adapter.Install(path)
	if err != nil {
		return err
	}
	if rp != nil {
		pkg.Source = rp.Source
		if pkg.Description == "" {
			pkg.Description = rp.Description
		}
	} else if pkg.Source == "" {
		pkg.Source = core.SourceForeign
	}

	if problems, err := abi.CheckStagedFiles(m.cfg.RootDir, files); err != nil {
		ui.Warn("glibc uyumluluk kontrolü çalıştırılamadı, kurulum yine de devam ediyor: %v", err)
	} else if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = "  " + p.String()
		}
		msg := fmt.Sprintf("%s hedefin glibc'siyle uyumsuz:\n%s", pkg.Name, strings.Join(lines, "\n"))
		if !m.Force {
			return fmt.Errorf("%s\nkurulum iptal edildi (üzerine yazılırsa çalışan bir sistem dosyasını bozabilirdi) - riski göze alıp yine de kurmak için --force ekleyin", msg)
		}
		ui.Warn("%s\n--force ile kurulumu yine de sürdürüyorum, sorumluluk sizde", msg)
	}

	if err := m.installer.CheckConflicts(pkg.Name, files); err != nil {
		return err
	}
	if err := m.installer.Commit(pkg, files); err != nil {
		return err
	}
	ui.Ok("%s %s kuruldu %s", pkg.Name, pkg.Version, ui.Tag(string(pkg.Source)))
	return nil
}

func (m *Manager) Remove(name string) error {
	pkg, err := m.db.Get(name)
	if err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("%s kurulu değil", name)
	}
	if err := m.installer.Uninstall(name); err != nil {
		return err
	}
	ui.Ok("%s kaldırıldı", name)
	return nil
}

func (m *Manager) Search(query string) ([]core.RemotePackage, error) {
	var all []core.RemotePackage
	for _, src := range m.sources {
		results, err := src.Search(query)
		if err != nil {
			ui.Warn("%s içinde arama başarısız: %v", src.Name(), err)
			continue
		}
		all = append(all, results...)
	}
	return all, nil
}

func (m *Manager) Info(name string) (*core.Package, *core.RemotePackage, error) {
	installed, err := m.db.Get(name)
	if err != nil {
		return nil, nil, err
	}
	if installed != nil {
		return installed, nil, nil
	}
	rp, err := m.resolve("", name)
	if err != nil {
		return nil, nil, err
	}
	return nil, rp, nil
}

func (m *Manager) List() ([]*core.Package, error) {
	return m.db.List()
}

func (m *Manager) Upgrade() error {
	installed, err := m.db.List()
	if err != nil {
		return err
	}
	upgraded := 0
	for _, pkg := range installed {
		if pkg.Source == core.SourceForeign {
			continue
		}
		rp, err := m.resolve(sourceTag(pkg.Source), pkg.Name)
		if err != nil || rp == nil {
			continue
		}
		if rp.Version == pkg.Version {
			continue
		}
		ui.Info("yükseltiliyor: %s %s -> %s", pkg.Name, pkg.Version, rp.Version)
		if err := m.installRemote(rp, map[string]bool{}); err != nil {
			ui.Error("%s yükseltilemedi: %v", pkg.Name, err)
			continue
		}
		upgraded++
	}
	if upgraded == 0 {
		ui.Ok("her şey güncel")
	} else {
		ui.Ok("%d paket yükseltildi", upgraded)
	}
	return nil
}
