//go:build !windows

package winsys

import "errors"

type Service struct {
	Name    string
	Display string
}

var errWindows = errors.New("only on Windows")

func IsElevated() bool                   { return false }
func Processes() (map[string]int, error) { return nil, errWindows }
func ActiveServices() ([]Service, error) { return nil, errWindows }
