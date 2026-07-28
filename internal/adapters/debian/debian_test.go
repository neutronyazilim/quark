package debian

import "testing"

func TestParseControl(t *testing.T) {
	text := `Package: curl
Version: 7.88.1-10+deb12u5
Depends: libc6 (>= 2.34), libssl3 (>= 3.0.0) | libssl1.1
Description: command line tool for transferring data
 extended description continues here
`
	pkg := parseControl(text)
	if pkg.Name != "curl" {
		t.Errorf("Name = %q, want curl", pkg.Name)
	}
	if pkg.Version != "7.88.1-10+deb12u5" {
		t.Errorf("Version = %q", pkg.Version)
	}
	want := []string{"libc6", "libssl3"}
	if len(pkg.Depends) != len(want) {
		t.Fatalf("Depends = %v, want %v", pkg.Depends, want)
	}
	for i, w := range want {
		if pkg.Depends[i] != w {
			t.Errorf("Depends[%d] = %q, want %q", i, pkg.Depends[i], w)
		}
	}
}

func TestSplitDepends(t *testing.T) {
	got := splitDepends("libc6 (>= 2.34), libssl3 | libssl1.1, zlib1g")
	want := []string{"libc6", "libssl3", "zlib1g"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestDetect(t *testing.T) {
	a := New(t.TempDir())
	if !a.Detect("curl_7.88.1-10_amd64.deb") {
		t.Error("Detect() = false for a .deb path")
	}
	if a.Detect("curl.pkg.tar.zst") {
		t.Error("Detect() = true for a .pkg.tar.zst path")
	}
}
