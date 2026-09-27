package winws

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func Prepare() error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return errors.New("administrator rights required: run the terminal as administrator")
	}

	if out, err := exec.Command("netsh", "interface", "tcp", "set", "global", "timestamps=enabled").CombinedOutput(); err != nil {
		return fmt.Errorf("enable tcp timestamps: %v: %s", err, out)
	}
	return nil
}

func Run(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	signal.Ignore(os.Interrupt)
	return cmd.Run()
}
