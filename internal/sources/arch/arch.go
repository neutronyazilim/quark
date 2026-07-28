package arch

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/fetch"
)

type dbEntry struct {
	Name     string
	Version  string
	Desc     string
	Filename string
	SHA256   string
	Depends  []string
	Repo     string
}

type Source struct {
	cfg      config.ArchConfig
	cacheDir string
	index    map[string]dbEntry
}

func New(cfg config.ArchConfig, cacheDir string) *Source {
	return &Source{cfg: cfg, cacheDir: filepath.Join(cacheDir, "arch")}
}

func (s *Source) Name() string { return "arch" }

func (s *Source) dbPath(repo string) string {
	return filepath.Join(s.cacheDir, repo+".db.tar.gz")
}

func (s *Source) Sync() error {
	for _, repo := range s.cfg.Repos {
		url := fmt.Sprintf("%s/%s/os/%s/%s.db",
			strings.TrimRight(s.cfg.Mirror, "/"), repo, s.cfg.Arch, repo)
		if err := fetch.Download(url, s.dbPath(repo), fmt.Sprintf("arch/%s db", repo), ""); err != nil {
			return fmt.Errorf("arch reposu senkronize edilemedi (%s): %w", repo, err)
		}
	}
	return s.loadIndex()
}

func (s *Source) loadIndex() error {
	index := map[string]dbEntry{}
	for _, repo := range s.cfg.Repos {
		entries, err := parseDB(s.dbPath(repo), repo)
		if err != nil {
			return fmt.Errorf("arch/%s db önce senkronize edilmeli: `quark :u` çalıştırın (%w)", repo, err)
		}
		for _, e := range entries {
			if _, exists := index[e.Name]; !exists {
				index[e.Name] = e
			}
		}
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

func parseDB(path, repo string) ([]dbEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var entries []dbEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(hdr.Name, "/desc") {
			continue
		}
		var buf strings.Builder
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, err
		}
		e := parseDesc(buf.String())
		e.Repo = repo
		entries = append(entries, e)
	}
	return entries, nil
}

func parseDesc(text string) dbEntry {
	var e dbEntry
	blocks := strings.Split(text, "\n\n")
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 1 || !strings.HasPrefix(lines[0], "%") {
			continue
		}
		key := strings.Trim(lines[0], "%")
		values := lines[1:]
		switch key {
		case "NAME":
			if len(values) > 0 {
				e.Name = values[0]
			}
		case "VERSION":
			if len(values) > 0 {
				e.Version = values[0]
			}
		case "DESC":
			if len(values) > 0 {
				e.Desc = values[0]
			}
		case "FILENAME":
			if len(values) > 0 {
				e.Filename = values[0]
			}
		case "SHA256SUM":
			if len(values) > 0 {
				e.SHA256 = values[0]
			}
		case "DEPENDS":
			for _, v := range values {

				name := v
				for _, op := range []string{">=", "<=", "==", ">", "<", "="} {
					if i := strings.Index(name, op); i >= 0 {
						name = name[:i]
						break
					}
				}
				e.Depends = append(e.Depends, name)
			}
		}
	}
	return e
}

func (s *Source) Search(query string) ([]core.RemotePackage, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []core.RemotePackage
	for _, e := range s.index {
		if strings.Contains(strings.ToLower(e.Name), q) || strings.Contains(strings.ToLower(e.Desc), q) {
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

func (s *Source) toRemote(e dbEntry) core.RemotePackage {
	url := fmt.Sprintf("%s/%s/os/%s/%s",
		strings.TrimRight(s.cfg.Mirror, "/"), e.Repo, s.cfg.Arch, e.Filename)
	return core.RemotePackage{
		Name:        e.Name,
		Version:     e.Version,
		Description: e.Desc,
		Format:      core.FormatArch,
		Source:      core.SourceArch,
		URL:         url,
		SHA256:      e.SHA256,
		Depends:     e.Depends,
	}
}

func (s *Source) PackageCacheDir() string {
	return filepath.Join(s.cacheDir, "packages")
}
