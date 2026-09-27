package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"nova/internal/app"
	"nova/internal/diag"
	"nova/internal/winsys"
)

func cmdDiag(a *app.App, argv []string) error {
	fs := flag.NewFlagSet("diag", flag.ContinueOnError)
	fix := fs.Bool("fix", false, "repair what can be repaired (removes conflicting services, unloads a stale driver)")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	checks := diag.Run(diag.Env{Home: a.Home, BinDir: a.BinDir(), HostsPath: hostsPath()})
	problems, fixable := 0, 0
	for _, c := range checks {
		mark := map[diag.Level]string{diag.OK: "[ok]  ", diag.Warn: "[warn]", diag.Fail: "[FAIL]"}[c.Level]
		fmt.Printf("%s %-36s %s\n", mark, c.Name, c.Detail)
		if c.Hint != "" {
			fmt.Printf("       %s\n", c.Hint)
		}
		if c.Level != diag.OK {
			problems++
		}
		if c.Fix != nil {
			fixable++
		}
	}
	fmt.Println()
	switch {
	case problems == 0:
		fmt.Println("no problems found")
	case fixable > 0 && !*fix:
		fmt.Printf("%d problem(s), %d fixable: nova diag --fix\n", problems, fixable)
	case !*fix:
		fmt.Printf("%d problem(s)\n", problems)
	}
	if !*fix || fixable == 0 {
		return nil
	}
	if !winsys.IsElevated() {
		return fmt.Errorf("--fix needs administrator rights")
	}
	for _, c := range checks {
		if c.Fix == nil {
			continue
		}
		if err := c.Fix(); err != nil {
			fmt.Printf("fix %s: %v\n", c.Name, err)
		} else {
			fmt.Printf("fixed: %s\n", c.Name)
		}
	}
	return nil
}

func hostsPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}
