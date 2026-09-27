//go:build !windows

package service

import "errors"

const Name = "nova"

var Foreign = []string{"zapret", "GoodbyeDPI", "discordfix_zapret", "winws1", "winws2"}

var DriverServices = []string{"WinDivert", "WinDivert14"}

var errWindows = errors.New("services are supported only on Windows")

type Status struct {
	Installed bool
	State     string
	Command   string
	Foreign   []string
}

func Install(exe string, args []string) error { return errWindows }
func Remove() error                           { return errWindows }
func Query() (Status, error)                  { return Status{}, errWindows }
func State(name string) (string, error)       { return "", errWindows }
func Stop(name string) (bool, error)          { return false, errWindows }
func Start(name string) error                 { return errWindows }
func Delete(name string) error                { return errWindows }
