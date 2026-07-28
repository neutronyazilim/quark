package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

var colorEnabled = isTerminal(os.Stdout)

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func paint(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + reset
}

func Info(format string, args ...any) {
	fmt.Printf("%s %s\n", paint(bold+blue, "==>"), fmt.Sprintf(format, args...))
}

func Ok(format string, args ...any) {
	fmt.Printf("%s %s\n", paint(bold+green, "✓"), fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", paint(bold+yellow, "!"), fmt.Sprintf(format, args...))
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", paint(bold+red, "✗"), fmt.Sprintf(format, args...))
}

func Muted(format string, args ...any) {
	fmt.Printf("  %s\n", paint(gray, fmt.Sprintf(format, args...)))
}

func Tag(source string) string {
	return paint(cyan, "["+source+"]")
}

type ProgressBar struct {
	Label     string
	Total     int64
	current   int64
	lastPrint time.Time
	out       io.Writer
}

func NewProgressBar(label string, total int64) *ProgressBar {
	return &ProgressBar{Label: label, Total: total, out: os.Stdout}
}

func (p *ProgressBar) Update(n int64) {
	p.current += n

	if time.Since(p.lastPrint) < 50*time.Millisecond && p.current < p.Total {
		return
	}
	p.lastPrint = time.Now()
	p.draw()
}

func (p *ProgressBar) draw() {
	const width = 30
	var frac float64
	if p.Total > 0 {
		frac = float64(p.current) / float64(p.Total)
		if frac > 1 {
			frac = 1
		}
	}
	filled := int(frac * width)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)

	label := p.Label
	if !colorEnabled {
		if p.Total > 0 {
			fmt.Fprintf(p.out, "\r%s [%s] %5.1f%% (%s/%s)", label, bar, frac*100, humanBytes(p.current), humanBytes(p.Total))
		} else {
			fmt.Fprintf(p.out, "\r%s %s", label, humanBytes(p.current))
		}
		return
	}
	if p.Total > 0 {
		fmt.Fprintf(p.out, "\r%s %s[%s]%s %5.1f%% %s(%s/%s)%s",
			label, cyan, bar, reset, frac*100, gray, humanBytes(p.current), humanBytes(p.Total), reset)
	} else {
		fmt.Fprintf(p.out, "\r%s %s%s%s", label, gray, humanBytes(p.current), reset)
	}
}

func (p *ProgressBar) Finish() {
	p.current = p.Total
	p.draw()
	fmt.Fprintln(p.out)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
