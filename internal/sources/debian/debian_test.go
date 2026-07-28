package debian

import (
	"strings"
	"testing"
)

func TestParsePackages(t *testing.T) {
	text := `Package: curl
Version: 7.88.1-10
Filename: pool/main/c/curl/curl_7.88.1-10_amd64.deb
SHA256: abc123
Depends: libc6 (>= 2.34), libssl3
Description: command line tool for transferring data

Package: wget
Version: 1.21.3-1
Filename: pool/main/w/wget/wget_1.21.3-1_amd64.deb
SHA256: def456
Description: retrieves files from the web
`
	entries, err := parsePackages(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Name != "curl" || entries[0].Version != "7.88.1-10" {
		t.Errorf("entries[0] = %+v", entries[0])
	}
	if entries[0].Filename != "pool/main/c/curl/curl_7.88.1-10_amd64.deb" {
		t.Errorf("Filename = %q", entries[0].Filename)
	}
	if len(entries[0].Depends) != 2 {
		t.Errorf("Depends = %v, want 2 entries", entries[0].Depends)
	}
	if entries[1].Name != "wget" {
		t.Errorf("entries[1].Name = %q, want wget", entries[1].Name)
	}
}
