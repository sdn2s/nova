package diag

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type Level int

const (
	OK Level = iota
	Warn
	Fail
)

func (l Level) String() string {
	switch l {
	case OK:
		return "ok"
	case Warn:
		return "warn"
	default:
		return "FAIL"
	}
}

type Check struct {
	Name   string
	Level  Level
	Detail string
	Hint   string
	Fix    func() error
}

type Env struct {
	Home      string
	BinDir    string
	HostsPath string
}

var Required = []string{"winws.exe", "cygwin1.dll", "WinDivert.dll", "WinDivert64.sys"}

func Run(env Env) []Check {
	checks := []Check{
		checkBinaries(env.BinDir),
		checkPath(env.Home, os.Getenv("OneDrive")),
	}
	if env.HostsPath != "" {
		checks = append(checks, checkHosts(env.HostsPath))
	}
	return append(checks, platformChecks(env)...)
}

func checkBinaries(binDir string) Check {
	c := Check{Name: "binaries"}
	var missing []string
	for _, f := range Required {
		if _, err := os.Stat(filepath.Join(binDir, f)); err != nil {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		c.Level = Fail
		c.Detail = "missing: " + strings.Join(missing, ", ")
		c.Hint = "antivirus may have quarantined them; add the nova folder to exclusions and restore the files"
		return c
	}
	bad, err := verifySums(binDir)
	switch {
	case err != nil:
		c.Level = Warn
		c.Detail = "checksums not verified: " + err.Error()
	case len(bad) > 0:
		c.Level = Fail
		c.Detail = "checksum mismatch: " + strings.Join(bad, ", ")
		c.Hint = "files were modified; reinstall bin/ from the release"
	default:
		c.Detail = "present, checksums match"
	}
	return c
}

func verifySums(binDir string) ([]string, error) {
	f, err := os.Open(filepath.Join(binDir, "SHA256SUMS"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var bad []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		want, name := fields[0], strings.TrimPrefix(fields[1], "*")
		got, err := fileSHA256(filepath.Join(binDir, name))
		if err != nil || !strings.EqualFold(got, want) {
			bad = append(bad, name)
		}
	}
	return bad, sc.Err()
}

func fileSHA256(path string) (string, error) {
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

func checkPath(home, oneDrive string) Check {
	c := Check{Name: "install path", Detail: home}
	lower := strings.ToLower(home)
	if oneDrive != "" && strings.HasPrefix(lower, strings.ToLower(oneDrive)) || strings.Contains(lower, `\onedrive`) {
		c.Level = Fail
		c.Hint = `OneDrive folders break WinDivert file access; move nova to e.g. C:\nova`
		return c
	}
	for _, r := range home {
		if unicode.Is(unicode.Cyrillic, r) {
			c.Level = Warn
			c.Hint = `the path contains Cyrillic; if bypass does not work move nova to e.g. C:\nova`
			return c
		}
	}
	return c
}

func checkHosts(path string) Check {
	c := Check{Name: "hosts file"}
	f, err := os.Open(path)
	if err != nil {
		c.Detail = "not readable, skipped"
		return c
	}
	defer f.Close()
	var hits []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		l := strings.ToLower(line)
		if strings.Contains(l, "youtube.com") || strings.Contains(l, "youtu.be") {
			hits = append(hits, line)
		}
	}
	if len(hits) > 0 {
		c.Level = Warn
		c.Detail = fmt.Sprintf("%d YouTube entries, e.g. %q", len(hits), hits[0])
		c.Hint = "hosts overrides for YouTube usually point to dead mirrors; remove them"
	} else {
		c.Detail = "no YouTube overrides"
	}
	return c
}
