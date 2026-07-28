package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/neutronyazilim/quark/internal/core"
	"github.com/ulikunitz/xz"
)

type Adapter struct {
	StagingRoot string
}

func New(stagingRoot string) *Adapter { return &Adapter{StagingRoot: stagingRoot} }

func (a *Adapter) Format() core.Format { return core.FormatDebian }

func (a *Adapter) Detect(path string) bool {
	return strings.HasSuffix(path, ".deb")
}

func (a *Adapter) Install(path string) (*core.Package, []core.StagedFile, error) {
	arEntries, err := readAr(path)
	if err != nil {
		return nil, nil, fmt.Errorf(".deb açılamadı %s: %w - dosya bozuk olabilir, yeniden indirin", path, err)
	}

	var controlData, archiveData []byte
	var dataName string
	for name, data := range arEntries {
		switch {
		case strings.HasPrefix(name, "control.tar"):
			controlData = data
		case strings.HasPrefix(name, "data.tar"):
			archiveData = data
			dataName = name
		}
	}
	if controlData == nil {
		return nil, nil, fmt.Errorf("%s içinde control.tar.* yok - geçerli bir .deb değil", path)
	}
	if archiveData == nil {
		return nil, nil, fmt.Errorf("%s içinde data.tar.* yok - geçerli bir .deb değil", path)
	}

	controlText, err := extractControlFile(controlData)
	if err != nil {
		return nil, nil, fmt.Errorf("control dosyası okunamadı: %w", err)
	}
	pkg := parseControl(controlText)
	pkg.Format = core.FormatDebian
	pkg.Source = core.SourceDebian

	stageDir := filepath.Join(a.StagingRoot, fmt.Sprintf("deb-%d", os.Getpid()))
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return nil, nil, err
	}
	files, err := extractDataTar(archiveData, dataName, stageDir)
	if err != nil {
		return nil, nil, fmt.Errorf("data.tar açılamadı: %w", err)
	}

	return pkg, files, nil
}

func readAr(path string) (map[string][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	magic := make([]byte, 8)
	if _, err := io.ReadFull(f, magic); err != nil {
		return nil, err
	}
	if string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("ar magic bulunamadı")
	}

	entries := map[string][]byte{}
	header := make([]byte, 60)
	for {
		_, err := io.ReadFull(f, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimRight(string(header[0:16]), " ")
		name = strings.TrimSuffix(name, "/")
		sizeStr := strings.TrimSpace(string(header[48:58]))
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ar boyutu ayrıştırılamadı %q: %w", sizeStr, err)
		}

		data := make([]byte, size)
		if _, err := io.ReadFull(f, data); err != nil {
			return nil, err
		}
		entries[name] = data

		if size%2 == 1 {
			f.Seek(1, io.SeekCurrent)
		}
	}
	return entries, nil
}

func decompress(name string, data []byte) (io.Reader, error) {
	r := bytes.NewReader(data)
	switch {
	case strings.HasSuffix(name, ".gz"):
		return gzip.NewReader(r)
	case strings.HasSuffix(name, ".xz"):
		return xz.NewReader(r)
	case strings.HasSuffix(name, ".zst"):
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return zr.IOReadCloser(), nil
	case strings.HasSuffix(name, ".tar"):
		return r, nil
	}
	return nil, fmt.Errorf("bilinmeyen sıkıştırma: %s", name)
}

func extractControlFile(controlTarData []byte) (string, error) {

	for _, suffix := range []string{".gz", ".xz", ".zst", ""} {
		r, err := decompress("control.tar"+suffix, controlTarData)
		if err != nil {
			continue
		}
		tr := tar.NewReader(r)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if strings.TrimPrefix(hdr.Name, "./") == "control" {
				var buf strings.Builder
				io.Copy(&buf, tr)
				return buf.String(), nil
			}
		}
	}
	return "", fmt.Errorf("control.tar içinde 'control' dosyası bulunamadı")
}

func extractDataTar(data []byte, arName, stageDir string) ([]core.StagedFile, error) {
	suffix := arName[strings.Index(arName, ".tar")+4:]
	r, err := decompress("data.tar"+suffix, data)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(r)

	var files []core.StagedFile
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || name == "." {
			continue
		}
		stagedPath := filepath.Join(stageDir, filepath.FromSlash(name))

		switch hdr.Typeflag {
		case tar.TypeDir:
			files = append(files, core.StagedFile{RelPath: name, IsDir: true, Mode: os.FileMode(hdr.Mode)})
		case tar.TypeSymlink:
			files = append(files, core.StagedFile{RelPath: name, IsLink: true, LinkTo: hdr.Linkname})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
				return nil, err
			}
			out, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return nil, err
			}
			out.Close()
			files = append(files, core.StagedFile{RelPath: name, SrcPath: stagedPath, Mode: os.FileMode(hdr.Mode)})
		}
	}
	return files, nil
}

func parseControl(text string) *core.Package {
	pkg := &core.Package{}
	lines := strings.Split(text, "\n")
	var curKey string
	for _, line := range lines {
		if line == "" {
			continue
		}
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && curKey != "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		curKey = key
		switch key {
		case "Package":
			pkg.Name = val
		case "Version":
			pkg.Version = val
		case "Description":
			pkg.Description = val
		case "Depends":
			pkg.Depends = splitDepends(val)
		}
	}
	return pkg
}

func splitDepends(val string) []string {
	var out []string
	for _, entry := range strings.Split(val, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		alt := strings.SplitN(entry, "|", 2)[0]
		alt = strings.TrimSpace(alt)
		if i := strings.Index(alt, "("); i >= 0 {
			alt = strings.TrimSpace(alt[:i])
		}
		if alt != "" {
			out = append(out, alt)
		}
	}
	return out
}
