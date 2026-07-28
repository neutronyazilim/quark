package abi

import (
	"debug/elf"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Incompatibility struct {
	RelPath         string
	RequiredVersion string
	MaxAvailable    string
}

func (i Incompatibility) String() string {
	return fmt.Sprintf("%s: %s gerekiyor, hedefte en fazla %s var", i.RelPath, i.RequiredVersion, i.MaxAvailable)
}

func ProvidedGlibcVersions(libcPath string) (map[string]bool, error) {
	f, err := elf.Open(libcPath)
	if err != nil {
		return nil, fmt.Errorf("libc.so.6 açılamadı %s: %w", libcPath, err)
	}
	defer f.Close()

	syms, err := f.DynamicSymbols()
	if err != nil {
		return nil, fmt.Errorf("libc.so.6 sembol tablosu okunamadı: %w", err)
	}

	versions := map[string]bool{}
	for _, s := range syms {
		if s.Version != "" && strings.HasPrefix(s.Version, "GLIBC_") {
			versions[s.Version] = true
		}
	}
	return versions, nil
}

func maxVersion(versions map[string]bool) string {
	max := ""
	for v := range versions {
		if max == "" || compareGlibcVersions(v, max) > 0 {
			max = v
		}
	}
	return max
}

func compareGlibcVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(a, "GLIBC_"), ".")
	pb := strings.Split(strings.TrimPrefix(b, "GLIBC_"), ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		na, nb := 0, 0
		if i < len(pa) {
			na, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			nb, _ = strconv.Atoi(pb[i])
		}
		if na != nb {
			return na - nb
		}
	}
	return 0
}

func CheckFile(path string, provided map[string]bool) ([]Incompatibility, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	magic := make([]byte, 4)
	if n, _ := f.Read(magic); n < 4 || string(magic) != "\x7fELF" {
		return nil, nil
	}

	ef, err := elf.NewFile(f)
	if err != nil {

		return nil, nil
	}
	defer ef.Close()

	imports, err := ef.ImportedSymbols()
	if err != nil {
		return nil, nil
	}

	seen := map[string]bool{}
	var problems []Incompatibility
	maxProvided := maxVersion(provided)
	for _, sym := range imports {
		if !strings.HasPrefix(sym.Version, "GLIBC_") || seen[sym.Version] {
			continue
		}
		seen[sym.Version] = true
		if provided[sym.Version] {
			continue
		}
		problems = append(problems, Incompatibility{
			RelPath:         path,
			RequiredVersion: sym.Version,
			MaxAvailable:    maxProvided,
		})
	}
	return problems, nil
}

func SortIncompatibilities(problems []Incompatibility) {
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].RelPath != problems[j].RelPath {
			return problems[i].RelPath < problems[j].RelPath
		}
		return problems[i].RequiredVersion < problems[j].RequiredVersion
	})
}
