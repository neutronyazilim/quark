package core

import "time"

type Format string

const (
	FormatNative  Format = "qpk"
	FormatArch    Format = "pkg.tar.zst"
	FormatDebian  Format = "deb"
	FormatGeneric Format = "tar"
)

type Source string

const (
	SourceNative  Source = "native"
	SourceArch    Source = "arch"
	SourceDebian  Source = "debian"
	SourceForeign Source = "foreign"
)

type Package struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	Format      Format    `json:"format"`
	Source      Source    `json:"source"`
	Depends     []string  `json:"depends,omitempty"`
	Files       []string  `json:"files"`
	SHA256      string    `json:"sha256,omitempty"`
	InstalledAt time.Time `json:"installed_at"`
	Size        int64     `json:"size,omitempty"`
}

func (p *Package) Tag() string {
	return "[" + string(p.Source) + "]"
}
