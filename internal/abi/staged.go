package abi

import (
	"fmt"
	"path/filepath"

	"github.com/neutronyazilim/quark/internal/core"
)

func CheckStagedFiles(root string, files []core.StagedFile) ([]Incompatibility, error) {
	libcPath := filepath.Join(root, "usr/lib/libc.so.6")
	provided, err := ProvidedGlibcVersions(libcPath)
	if err != nil {

		return nil, nil
	}

	var all []Incompatibility
	for _, f := range files {
		if f.IsDir || f.IsLink || f.SrcPath == "" {
			continue
		}
		problems, err := CheckFile(f.SrcPath, provided)
		if err != nil {
			return nil, fmt.Errorf("%s incelenemedi: %w", f.RelPath, err)
		}
		for i := range problems {
			problems[i].RelPath = f.RelPath
		}
		all = append(all, problems...)
	}
	SortIncompatibilities(all)
	return all, nil
}
