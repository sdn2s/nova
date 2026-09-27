package diag

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeBin(t *testing.T, dir string, sums bool) {
	t.Helper()
	var list string
	for _, f := range Required {
		data := []byte("content of " + f)
		if err := os.WriteFile(filepath.Join(dir, f), data, 0o644); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		list += hex.EncodeToString(h[:]) + "  " + f + "\n"
	}
	if sums {
		os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(list), 0o644)
	}
}

func TestBinaries(t *testing.T) {
	dir := t.TempDir()
	if c := checkBinaries(dir); c.Level != Fail {
		t.Fatalf("empty dir: %v %s", c.Level, c.Detail)
	}
	writeBin(t, dir, true)
	if c := checkBinaries(dir); c.Level != OK {
		t.Fatalf("good: %v %s", c.Level, c.Detail)
	}
	os.WriteFile(filepath.Join(dir, "winws.exe"), []byte("tampered"), 0o644)
	if c := checkBinaries(dir); c.Level != Fail || c.Detail != "checksum mismatch: winws.exe" {
		t.Fatalf("tampered: %v %s", c.Level, c.Detail)
	}
	os.Remove(filepath.Join(dir, "SHA256SUMS"))
	if c := checkBinaries(dir); c.Level != Warn {
		t.Fatalf("no sums: %v %s", c.Level, c.Detail)
	}
}

func TestPath(t *testing.T) {
	for _, tc := range []struct {
		home, onedrive string
		want           Level
	}{
		{`C:\nova`, `C:\Users\u\OneDrive`, OK},
		{`C:\Users\u\OneDrive\nova`, `C:\Users\u\OneDrive`, Fail},
		{`D:\OneDrive - Work\nova`, ``, Fail},
		{`C:\Users\Антон\nova`, ``, Warn},
	} {
		if c := checkPath(tc.home, tc.onedrive); c.Level != tc.want {
			t.Errorf("%s: got %v, want %v", tc.home, c.Level, tc.want)
		}
	}
}

func TestHosts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	os.WriteFile(p, []byte("# 1.2.3.4 youtube.com\n127.0.0.1 localhost\n"), 0o644)
	if c := checkHosts(p); c.Level != OK {
		t.Fatalf("commented: %v", c.Level)
	}
	os.WriteFile(p, []byte("1.2.3.4 www.youtube.com\n"), 0o644)
	if c := checkHosts(p); c.Level != Warn {
		t.Fatalf("override: %v", c.Level)
	}
}
