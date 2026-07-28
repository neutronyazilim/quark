package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/neutronyazilim/quark/internal/ui"
)

var Client = &http.Client{Timeout: 5 * time.Minute}

type ErrChecksumMismatch struct {
	URL      string
	Expected string
	Got      string
}

func (e *ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("sha256 uyuşmadı %s: beklenen %s, alınan %s - mirror bozuk olabilir, tekrar deneyin ya da mirror'ı değiştirin", e.URL, e.Expected, e.Got)
}

func Download(url, destPath, label, expectedSHA256 string) error {
	if expectedSHA256 != "" {
		if ok, _ := verifyFile(destPath, expectedSHA256); ok {
			ui.Muted("önbellekte: %s", filepath.Base(destPath))
			return nil
		}
	} else if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
		ui.Muted("önbellekte: %s", filepath.Base(destPath))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	resp, err := Client.Get(url)
	if err != nil {
		return fmt.Errorf("indirilemedi %s: %w - internet bağlantınızı veya mirror URL'sini kontrol edin", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("indirilemedi %s: HTTP %s - mirror'da bu paket/sürüm olmayabilir", url, resp.Status)
	}

	tmp := destPath + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	bar := ui.NewProgressBar(label, resp.ContentLength)
	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher, &progressWriter{bar: bar})

	_, copyErr := io.Copy(writer, resp.Body)
	bar.Finish()
	out.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return fmt.Errorf("indirme kesildi %s: %w", url, copyErr)
	}

	if expectedSHA256 != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != expectedSHA256 {
			os.Remove(tmp)
			return &ErrChecksumMismatch{URL: url, Expected: expectedSHA256, Got: got}
		}
	}

	return os.Rename(tmp, destPath)
}

func verifyFile(path, expectedSHA256 string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, nil
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == expectedSHA256, nil
}

func SHA256Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type progressWriter struct{ bar *ui.ProgressBar }

func (w *progressWriter) Write(p []byte) (int, error) {
	w.bar.Update(int64(len(p)))
	return len(p), nil
}
