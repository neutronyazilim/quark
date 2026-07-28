package native

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/neutronyazilim/quark/internal/config"
)

func TestSearchAndFind(t *testing.T) {
	cacheDir := t.TempDir()
	src := New(config.NativeConfig{Enabled: true, Mirror: "https://example.invalid"}, cacheDir)

	entries := []IndexEntry{
		{Name: "htop", Version: "3.3.0", Description: "process viewer", Filename: "htop-3.3.0.qpk", SHA256: "abc"},
		{Name: "neofetch", Version: "7.1.0", Description: "system info tool", Filename: "neofetch-7.1.0.qpk"},
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(src.indexPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src.indexPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := src.Find("htop")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("Find(htop) = nil")
	}
	if found.URL != "https://example.invalid/htop-3.3.0.qpk" {
		t.Errorf("URL = %q", found.URL)
	}
	if found.SHA256 != "abc" {
		t.Errorf("SHA256 = %q, want abc", found.SHA256)
	}

	results, err := src.Search("system")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "neofetch" {
		t.Errorf("Search(system) = %+v, want [neofetch]", results)
	}

	missing, err := src.Find("does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Errorf("Find(does-not-exist) = %+v, want nil", missing)
	}
}
