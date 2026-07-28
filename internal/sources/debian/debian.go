package debian

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/fetch"
	"github.com/ulikunitz/xz"
)

type pkgEntry struct {
	Name        string
	Version     string
	Description string
	Filename    string
	SHA256      string
	Depends     []string
}

type Source struct {
	cfg      config.DebianConfig
	cacheDir string
	index    map[string]pkgEntry
}

func New(cfg config.DebianConfig, cacheDir string) *Source {
	return &Source{cfg: cfg, cacheDir: filepath.Join(cacheDir, "debian")}
}

func (s *Source) Name() string { return "debian" }

func (s *Source) packagesPath(component, ext string) string {
	return filepath.Join(s.cacheDir, component+".Packages."+ext)
}

func (s *Source) Sync() error {
	for _, component := range s.cfg.Components {
		base := fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages",
			strings.TrimRight(s.cfg.Mirror, "/"), s.cfg.Suite, component, s.cfg.Arch)

		err := fetch.Download(base+".gz", s.packagesPath(component, "gz"),
			fmt.Sprintf("debian/%s index", component), "")
		if err != nil {

			if xzErr := fetch.Download(base+".xz", s.packagesPath(component, "xz"),
				fmt.Sprintf("debian/%s index", component), ""); xzErr != nil {
				return fmt.Errorf("debian reposu senkronize edilemedi (%s): %w", component, err)
			}
		}
	}
	return s.loadIndex()
}

func (s *Source) loadIndex() error {
	index := map[string]pkgEntry{}
	found := false
	for _, component := range s.cfg.Components {
		var r io.Reader
		var f *os.File
		var err error

		if f, err = os.Open(s.packagesPath(component, "gz")); err == nil {
			defer f.Close()
			r, err = gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("debian/%s index açılamadı: %w", component, err)
			}
		} else if f, err = os.Open(s.packagesPath(component, "xz")); err == nil {
			defer f.Close()
			r, err = xz.NewReader(f)
			if err != nil {
				return fmt.Errorf("debian/%s index açılamadı: %w", component, err)
			}
		} else {
			continue
		}
		found = true

		entries, err := parsePackages(r)
		if err != nil {
			return fmt.Errorf("debian/%s index ayrıştırılamadı: %w", component, err)
		}
		for _, e := range entries {
			if _, exists := index[e.Name]; !exists {
				index[e.Name] = e
			}
		}
	}
	if !found {
		return fmt.Errorf("debian index önce senkronize edilmeli: `quark :u` çalıştırın")
	}
	s.index = index
	return nil
}

func (s *Source) ensureLoaded() error {
	if s.index != nil {
		return nil
	}
	return s.loadIndex()
}

func parsePackages(r io.Reader) ([]pkgEntry, error) {
	var entries []pkgEntry
	var cur pkgEntry
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	flush := func() {
		if cur.Name != "" {
			entries = append(entries, cur)
		}
		cur = pkgEntry{}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Package":
			cur.Name = val
		case "Version":
			cur.Version = val
		case "Description":
			cur.Description = val
		case "Filename":
			cur.Filename = val
		case "SHA256":
			cur.SHA256 = val
		case "Depends":
			cur.Depends = splitDepends(val)
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func splitDepends(val string) []string {
	var out []string
	for _, entry := range strings.Split(val, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		alt := strings.SplitN(entry, "|", 2)[0]
		alt = strings.TrimSpace(alt)
		if i := strings.Index(alt, "("); i >= 0 {
			alt = strings.TrimSpace(alt[:i])
		}
		if alt != "" {
			out = append(out, alt)
		}
	}
	return out
}

func (s *Source) Search(query string) ([]core.RemotePackage, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []core.RemotePackage
	for _, e := range s.index {
		if strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(strings.ToLower(e.Description), q) {
			out = append(out, s.toRemote(e))
		}
	}
	return out, nil
}

func (s *Source) Find(name string) (*core.RemotePackage, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	e, ok := s.index[name]
	if !ok {
		return nil, nil
	}
	rp := s.toRemote(e)
	return &rp, nil
}

func (s *Source) toRemote(e pkgEntry) core.RemotePackage {
	return core.RemotePackage{
		Name:        e.Name,
		Version:     e.Version,
		Description: e.Description,
		Format:      core.FormatDebian,
		Source:      core.SourceDebian,
		URL:         strings.TrimRight(s.cfg.Mirror, "/") + "/" + e.Filename,
		SHA256:      e.SHA256,
		Depends:     e.Depends,
	}
}

func (s *Source) PackageCacheDir() string {
	return filepath.Join(s.cacheDir, "packages")
}
