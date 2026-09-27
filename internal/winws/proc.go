package winws

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Proc struct {
	cmd  *exec.Cmd
	done chan error
	log  *os.File
}

func Start(exe string, args []string, logPath string, settle time.Duration) (*Proc, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, err
	}
	log, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, err
	}
	p := &Proc{cmd: cmd, done: make(chan error, 1), log: log}
	go func() { p.done <- cmd.Wait() }()

	select {
	case err := <-p.done:
		log.Close()
		return nil, fmt.Errorf("exited at start (%v): %s", err, tail(logPath, 3))
	case <-time.After(settle):
		return p, nil
	}
}

func (p *Proc) Stop() {
	p.cmd.Process.Kill()
	<-p.done
	p.log.Close()
}

func tail(path string, n int) string {
	b, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(bytes.ReplaceAll(b, []byte("\r"), nil))), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}
