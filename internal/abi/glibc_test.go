package abi

import (
	"os"
	"testing"
)

func hostLibc(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/usr/lib/libc.so.6", "/lib/x86_64-linux-gnu/libc.so.6", "/lib64/libc.so.6"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no real libc.so.6 found on this machine to test against")
	return ""
}

func TestCompareGlibcVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"GLIBC_2.9", "GLIBC_2.28", -1},
		{"GLIBC_2.28", "GLIBC_2.9", 1},
		{"GLIBC_2.34", "GLIBC_2.34", 0},
		{"GLIBC_2.34", "GLIBC_2.34.1", -1},
	}
	for _, c := range cases {
		got := compareGlibcVersions(c.a, c.b)
		gotSign := 0
		switch {
		case got > 0:
			gotSign = 1
		case got < 0:
			gotSign = -1
		}
		if gotSign != c.want {
			t.Errorf("compareGlibcVersions(%q, %q) sign = %d, want %d", c.a, c.b, gotSign, c.want)
		}
	}
}

func TestProvidedGlibcVersions(t *testing.T) {
	libc := hostLibc(t)
	versions, err := ProvidedGlibcVersions(libc)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) == 0 {
		t.Fatal("no GLIBC_* versions found in a real libc.so.6")
	}

	if !versions["GLIBC_2.2.5"] {
		t.Errorf("expected GLIBC_2.2.5 to be provided, got %v", versions)
	}
}

func TestCheckFile_CompatibleAgainstItself(t *testing.T) {
	libc := hostLibc(t)
	provided, err := ProvidedGlibcVersions(libc)
	if err != nil {
		t.Fatal(err)
	}

	problems, err := CheckFile(libc, provided)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("real, working libc.so.6 reported as incompatible with itself: %v", problems)
	}
}

func TestCheckFile_DetectsMissingVersion(t *testing.T) {
	libc := hostLibc(t)
	real, err := ProvidedGlibcVersions(libc)
	if err != nil {
		t.Fatal(err)
	}

	limited := map[string]bool{}
	for v := range real {
		if compareGlibcVersions(v, "GLIBC_2.10") <= 0 {
			limited[v] = true
		}
	}

	problems, err := CheckFile(libc, limited)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 {
		t.Fatal("expected incompatibilities against an artificially old glibc, got none")
	}
	for _, p := range problems {
		if limited[p.RequiredVersion] {
			t.Errorf("flagged %q as missing but it IS in the limited provided set", p.RequiredVersion)
		}
	}
}

func TestCheckFile_NonELF(t *testing.T) {
	path := t.TempDir() + "/not-an-elf.txt"
	if err := os.WriteFile(path, []byte("just some text, not a binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := CheckFile(path, map[string]bool{"GLIBC_2.30": true})
	if err != nil {
		t.Fatal(err)
	}
	if problems != nil {
		t.Errorf("non-ELF file should never be flagged, got %v", problems)
	}
}
