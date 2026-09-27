//go:build !windows

package winws

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartStop(t *testing.T) {
	log := filepath.Join(t.TempDir(), "w.log")
	p, err := Start("/bin/sleep", []string{"30"}, log, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return")
	}
}

func TestStartEarlyExit(t *testing.T) {
	log := filepath.Join(t.TempDir(), "w.log")
	_, err := Start("/bin/sh", []string{"-c", "echo line1; echo bad option; exit 3"}, log, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "bad option") {
		t.Fatalf("got %v", err)
	}
}
