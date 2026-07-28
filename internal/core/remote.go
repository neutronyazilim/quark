package core

type RemotePackage struct {
	Name        string
	Version     string
	Description string
	Format      Format
	Source      Source

	URL string

	SHA256  string
	Depends []string
}

type SourceProvider interface {
	Name() string

	Sync() error

	Search(query string) ([]RemotePackage, error)

	Find(name string) (*RemotePackage, error)
}

type FormatAdapter interface {
	Format() Format

	Detect(path string) bool

	Install(path string) (*Package, []StagedFile, error)
}
