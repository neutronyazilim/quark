package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/config"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/neutronyazilim/quark/internal/pkgformat"
)

type mockSource struct {
	tag string
	pkg map[string]core.RemotePackage
}

func (s *mockSource) Name() string                                { return s.tag }
func (s *mockSource) Sync() error                                 { return nil }
func (s *mockSource) Search(string) ([]core.RemotePackage, error) { return nil, nil }
func (s *mockSource) Find(name string) (*core.RemotePackage, error) {
	if rp, ok := s.pkg[name]; ok {
		return &rp, nil
	}
	return nil, nil
}

func buildMockQpk(t *testing.T, dir, pkgName string) string {
	t.Helper()
	srcRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcRoot, "usr/bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "usr/bin", pkgName), []byte("bin:"+pkgName), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, pkgName+".qpk")
	if err := pkgformat.Build(srcRoot, pkgformat.Metadata{Name: pkgName, Version: "1.0"}, dest); err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestNonNativeDepDepthIsBounded(t *testing.T) {
	rootDir := t.TempDir()
	cacheDir := t.TempDir()
	artifactDir := t.TempDir()

	cfg := config.Default()
	cfg.RootDir = rootDir
	cfg.CacheDir = cacheDir
	cfg.Sources.Native.Enabled = false
	cfg.Sources.Arch.Enabled = false
	cfg.Sources.Debian.Enabled = false

	m, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockSource{tag: "arch", pkg: map[string]core.RemotePackage{}}
	for name, deps := range map[string][]string{
		"app":  {"libA"},
		"libA": {"libB"},
		"libB": {"libC"},
		"libC": nil,
	} {
		mock.pkg[name] = core.RemotePackage{
			Name:    name,
			Version: "1.0",
			Format:  core.FormatNative,
			Source:  core.SourceArch,
			URL:     "mock://" + name,
			Depends: deps,
		}
	}
	m.sourceByTag["arch"] = mock
	m.sources = append(m.sources, mock)

	for name := range mock.pkg {
		cacheFile := filepath.Join(cacheDir, string(core.SourceArch), "packages", filepath.Base("mock://"+name))
		if err := os.MkdirAll(filepath.Dir(cacheFile), 0o755); err != nil {
			t.Fatal(err)
		}
		built := buildMockQpk(t, artifactDir, name)
		data, err := os.ReadFile(built)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cacheFile, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := m.Install("arch:app"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	pkgs, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range pkgs {
		got[p.Name] = true
	}

	if !got["app"] {
		t.Errorf("app was not installed: %v", got)
	}
	if !got["libA"] {
		t.Errorf("direct dependency libA was not installed (depth 1 should still resolve): %v", got)
	}
	if got["libB"] {
		t.Errorf("transitive dependency libB WAS installed - depth cap did not hold: %v", got)
	}
	if got["libC"] {
		t.Errorf("transitive dependency libC WAS installed - depth cap did not hold: %v", got)
	}
}
