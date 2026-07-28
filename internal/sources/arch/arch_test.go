package arch

import "testing"

func TestParseDesc(t *testing.T) {
	text := `%FILENAME%
htop-3.3.0-1-x86_64.pkg.tar.zst

%NAME%
htop

%VERSION%
3.3.0-1

%DESC%
Interactive process viewer

%SHA256SUM%
abc123def456

%DEPENDS%
ncurses
glibc>=2.34
`
	e := parseDesc(text)
	if e.Name != "htop" {
		t.Errorf("Name = %q, want htop", e.Name)
	}
	if e.Version != "3.3.0-1" {
		t.Errorf("Version = %q", e.Version)
	}
	if e.Filename != "htop-3.3.0-1-x86_64.pkg.tar.zst" {
		t.Errorf("Filename = %q", e.Filename)
	}
	if e.SHA256 != "abc123def456" {
		t.Errorf("SHA256 = %q", e.SHA256)
	}
	want := []string{"ncurses", "glibc"}
	if len(e.Depends) != len(want) {
		t.Fatalf("Depends = %v, want %v", e.Depends, want)
	}
	for i, w := range want {
		if e.Depends[i] != w {
			t.Errorf("Depends[%d] = %q, want %q", i, e.Depends[i], w)
		}
	}
}
