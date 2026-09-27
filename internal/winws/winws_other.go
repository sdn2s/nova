//go:build !windows

package winws

import "errors"

var errWindows = errors.New("winws runs only on Windows")

func Prepare() error                      { return errWindows }
func Run(exe string, args []string) error { return errWindows }
