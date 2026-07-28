package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type DB struct {
	dir string
}

func OpenDB(rootDir string) (*DB, error) {
	dir := filepath.Join(rootDir, "var/lib/quark/installed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("quark db: %w", err)
	}
	return &DB{dir: dir}, nil
}

func (db *DB) recordPath(name string) string {
	return filepath.Join(db.dir, name+".json")
}

func (db *DB) Save(pkg *Package) error {
	data, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	tmp := db.recordPath(pkg.Name) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, db.recordPath(pkg.Name))
}

func (db *DB) Get(name string) (*Package, error) {
	data, err := os.ReadFile(db.recordPath(name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pkg Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("quark db: corrupt record for %s: %w", name, err)
	}
	return &pkg, nil
}

func (db *DB) Remove(name string) error {
	err := os.Remove(db.recordPath(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (db *DB) List() ([]*Package, error) {
	entries, err := os.ReadDir(db.dir)
	if err != nil {
		return nil, err
	}
	var pkgs []*Package
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(db.dir, e.Name()))
		if err != nil {
			continue
		}
		var pkg Package
		if err := json.Unmarshal(data, &pkg); err != nil {
			continue
		}
		pkgs = append(pkgs, &pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

func (db *DB) OwnerOf(path string) (string, error) {
	pkgs, err := db.List()
	if err != nil {
		return "", err
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			if f == path {
				return pkg.Name, nil
			}
		}
	}
	return "", nil
}
