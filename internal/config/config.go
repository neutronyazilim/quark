package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type NativeConfig struct {
	Enabled bool   `toml:"enabled"`
	Mirror  string `toml:"mirror"`
}

type ArchConfig struct {
	Enabled bool     `toml:"enabled"`
	Mirror  string   `toml:"mirror"`
	Repos   []string `toml:"repos"`
	Arch    string   `toml:"arch"`
}

type DebianConfig struct {
	Enabled    bool     `toml:"enabled"`
	Mirror     string   `toml:"mirror"`
	Suite      string   `toml:"suite"`
	Components []string `toml:"components"`
	Arch       string   `toml:"arch"`
}

type SourcesConfig struct {
	Native NativeConfig `toml:"native"`
	Arch   ArchConfig   `toml:"arch"`
	Debian DebianConfig `toml:"debian"`
}

type Config struct {
	Sources SourcesConfig `toml:"sources"`

	RootDir string `toml:"root_dir"`

	CacheDir string `toml:"cache_dir"`
}

func Default() *Config {
	return &Config{
		RootDir:  "/",
		CacheDir: "/var/cache/quark",
		Sources: SourcesConfig{
			Native: NativeConfig{
				Enabled: true,
				Mirror:  "https://repo.neutronlinux.org",
			},
			Arch: ArchConfig{
				Enabled: false,
				Mirror:  "https://geo.mirror.pkgbuild.com",
				Repos:   []string{"core", "extra"},
				Arch:    "x86_64",
			},
			Debian: DebianConfig{
				Enabled:    false,
				Mirror:     "http://deb.debian.org/debian",
				Suite:      "bookworm",
				Components: []string{"main"},
				Arch:       "amd64",
			},
		},
	}
}

const DefaultPath = "/etc/quark/quark.conf"

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config okunamadı %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("config ayrıştırılamadı %s: %w", path, err)
	}
	return cfg, nil
}
