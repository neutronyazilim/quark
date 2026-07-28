package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/fetch"
)

type IndexEntry struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	Depends     []string `json:"depends,omitempty"`
	Filename    string   `json:"filename"`
	SHA256      string   `json:"sha256"`
}

type Source struct {
	cfg      config.NativeConfig
	cacheDir string
	index    []IndexEntry
}

func New(cfg config.NativeConfig, cacheDir string) *Source {
	return &Source{cfg: cfg, cacheDir: filepath.Join(cacheDir, "native")}
}

func (s *Source) Name() string { return "native" }

func (s *Source) indexPath() string { return filepath.Join(s.cacheDir, "index.json") }

func (s *Source) Sync() error {
	url := strings.TrimRight(s.cfg.Mirror, "/") + "/index.json"
	if err := fetch.Download(url, s.indexPath(), "native index", ""); err != nil {
		return fmt.Errorf("native repo senkronize edilemedi: %w", err)
	}
	return s.loadIndex()
}

func (s *Source) loadIndex() error {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		return fmt.Errorf("native index önce senkronize edilmeli: `quark :u` çalıştırın (%w)", err)
	}
	var entries []IndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("native index ayrıştırılamadı: %w", err)
	}
	s.index = entries
	return nil
}

func (s *Source) ensureLoaded() error {
	if s.index != nil {
		return nil
	}
	return s.loadIndex()
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
	for _, e := range s.index {
		if e.Name == name {
			rp := s.toRemote(e)
			return &rp, nil
		}
	}
	return nil, nil
}

func (s *Source) toRemote(e IndexEntry) core.RemotePackage {
	filename := e.Filename
	if filename == "" {
		filename = e.Name + "-" + e.Version + ".qpk"
	}
	url := strings.TrimRight(s.cfg.Mirror, "/") + "/" + path.Base(filename)
	return core.RemotePackage{
		Name:        e.Name,
		Version:     e.Version,
		Description: e.Description,
		Format:      core.FormatNative,
		Source:      core.SourceNative,
		URL:         url,
		SHA256:      e.SHA256,
		Depends:     e.Depends,
	}
}

func (s *Source) PackageCacheDir() string {
	return filepath.Join(s.cacheDir, "packages")
}
