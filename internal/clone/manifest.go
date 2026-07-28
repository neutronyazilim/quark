package clone

import "time"

const ManifestVersion = 1

type PackageEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Format  string `json:"format"`
}

type Manifest struct {
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	Host      string         `json:"host,omitempty"`
	Packages  []PackageEntry `json:"packages"`

	Dotfiles []string `json:"dotfiles"`
}
