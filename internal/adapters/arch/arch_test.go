package arch

import "testing"

func TestParsePkgInfo(t *testing.T) {
	text := `pkgname = htop
pkgver = 3.3.0-1
pkgdesc = Interactive process viewer
depend = ncurses
depend = glibc>=2.34
`
	pkg := parsePkgInfo(text)
	if pkg.Name != "htop" {
		t.Errorf("Name = %q, want htop", pkg.Name)
	}
	if pkg.Version != "3.3.0-1" {
		t.Errorf("Version = %q, want 3.3.0-1", pkg.Version)
	}
	if pkg.Description != "Interactive process viewer" {
		t.Errorf("Description = %q", pkg.Description)
	}
	want := []string{"ncurses", "glibc"}
	if len(pkg.Depends) != len(want) {
		t.Fatalf("Depends = %v, want %v", pkg.Depends, want)
	}
	for i, w := range want {
		if pkg.Depends[i] != w {
			t.Errorf("Depends[%d] = %q, want %q", i, pkg.Depends[i], w)
		}
	}
}

func TestDetect(t *testing.T) {
	a := New(t.TempDir())
	if !a.Detect("htop-3.3.0-1-x86_64.pkg.tar.zst") {
		t.Error("Detect() = false for a .pkg.tar.zst path")
	}
	if a.Detect("htop.deb") {
		t.Error("Detect() = true for a .deb path")
	}
}
