package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nova/internal/app"
	"nova/internal/diag"
	"nova/internal/service"
	"nova/internal/winws"
)

var version = "dev"

const usage = `nova: DPI bypass manager for winws (zapret)

Usage: nova [--home DIR] <command> [args]

  list                        strategies (* = selected)
  show [strategy]             print the winws command line
  run [strategy]              run winws in this console (Ctrl+C to stop)
  service install [strategy]  install and start the Windows service
  service remove              stop and remove the service and the WinDivert driver
  service status              service state and conflicts
  diag [--fix]                find conflicts and environment problems
  probe [flags]               test strategies and pick the best one:
      --only a,b                test only these strategies
      --save | --install        save the best / also install the service with it
      --freeze                  add the TCP 16-20 KB check on hosting providers
      --freeze-host HOST        extra host for that check (e.g. your VPS)
      --timeout 5s --parallel 16 --keep-ipset
  config                      print settings
  set <key> <value>           change a setting: strategy, game_filter (off|tcp|udp|all),
                              game_tcp, game_udp, ipset (none|any|loaded)
  version
`

func main() {
	fs := flag.NewFlagSet("nova", flag.ExitOnError)
	home := fs.String("home", "", "install directory (default: next to nova.exe)")
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	fs.Parse(os.Args[1:])
	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		os.Exit(2)
	}
	if err := dispatch(*home, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func dispatch(home string, args []string) error {
	cmd, rest := args[0], args[1:]
	if cmd == "version" {
		fmt.Println(version)
		return nil
	}
	a, err := app.Open(home)
	if err != nil {
		return err
	}
	switch cmd {
	case "list":
		return cmdList(a)
	case "show":
		return cmdShow(a, arg(rest, 0))
	case "run":
		return cmdRun(a, arg(rest, 0))
	case "service":
		return cmdService(a, rest)
	case "diag":
		return cmdDiag(a, rest)
	case "probe":
		return cmdProbe(a, rest)
	case "config":
		fmt.Printf("home        %s\nstrategy    %s\ngame_filter %s (tcp %s, udp %s)\nipset       %s\n",
			a.Home, a.Settings.Strategy, a.Settings.GameFilter, a.Settings.GameTCP, a.Settings.GameUDP, a.Settings.Ipset)
		return nil
	case "set":
		if len(rest) != 2 {
			return fmt.Errorf("usage: nova set <key> <value>")
		}
		if rest[0] == "strategy" {
			if _, err := a.Strategy(rest[1]); err != nil {
				return err
			}
		}
		if err := a.Settings.Set(rest[0], rest[1]); err != nil {
			return err
		}
		if err := a.SaveSettings(); err != nil {
			return err
		}
		fmt.Println("saved; restart the service to apply: nova service install")
		return nil
	default:
		return fmt.Errorf("unknown command %q (nova --help)", cmd)
	}
}

func arg(a []string, i int) string {
	if i < len(a) {
		return a[i]
	}
	return ""
}

func cmdList(a *app.App) error {
	list, err := a.Strategies()
	if err != nil {
		return err
	}
	for _, s := range list {
		mark := " "
		if s.Name == a.Settings.Strategy {
			mark = "*"
		}
		fmt.Printf("%s %-22s %d profiles\n", mark, s.Name, len(s.Profiles))
	}
	return nil
}

func cmdShow(a *app.App, name string) error {
	s, r, err := a.Build(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "strategy %s, game_filter %s, ipset %s\n", s.Name, a.Settings.GameFilter, a.Settings.Ipset)
	for _, n := range r.Notes {
		fmt.Fprintln(os.Stderr, "  note:", n)
	}
	fmt.Print(quote(a.WinwsPath()))
	for _, x := range r.Args {
		if x == "--new" {
			fmt.Print(" ^\n  --new")
			continue
		}
		fmt.Print(" ", quote(x))
	}
	fmt.Println()
	return nil
}

func cmdRun(a *app.App, name string) error {
	s, r, err := a.Build(name)
	if err != nil {
		return err
	}
	if err := checkInstall(a); err != nil {
		return err
	}
	if err := winws.Prepare(); err != nil {
		return err
	}
	if st, err := service.Query(); err == nil && st.Installed && st.State == "running" {
		return fmt.Errorf("service %s is running; stop it first: nova service remove", service.Name)
	}
	fmt.Printf("running strategy %s, Ctrl+C to stop\n", s.Name)
	return winws.Run(a.WinwsPath(), r.Args)
}

func cmdService(a *app.App, rest []string) error {
	switch arg(rest, 0) {
	case "install":
		name := arg(rest, 1)
		s, r, err := a.Build(name)
		if err != nil {
			return err
		}
		if err := checkInstall(a); err != nil {
			return err
		}
		if err := winws.Prepare(); err != nil {
			return err
		}
		if st, err := service.Query(); err == nil && len(st.Foreign) > 0 {
			fmt.Printf("warning: conflicting services installed: %s. Remove them, they also use WinDivert\n", strings.Join(st.Foreign, ", "))
		}
		if err := service.Install(a.WinwsPath(), r.Args); err != nil {
			return err
		}
		if name != "" && name != a.Settings.Strategy {
			a.Settings.Strategy = name
			if err := a.SaveSettings(); err != nil {
				return err
			}
		}
		fmt.Printf("service %s running with strategy %s\n", service.Name, s.Name)
		return nil
	case "remove":
		if err := winws.Prepare(); err != nil {
			return err
		}
		if err := service.Remove(); err != nil {
			return err
		}
		fmt.Println("service removed")
		return nil
	case "status":
		st, err := service.Query()
		if err != nil {
			return err
		}
		if !st.Installed {
			fmt.Println("service: not installed")
		} else {
			fmt.Printf("service: %s (strategy in settings: %s)\n", st.State, a.Settings.Strategy)
		}
		if len(st.Foreign) > 0 {
			fmt.Printf("conflicting services: %s\n", strings.Join(st.Foreign, ", "))
		}
		return nil
	default:
		return fmt.Errorf("usage: nova service install [strategy] | remove | status")
	}
}

func checkInstall(a *app.App) error {
	for _, f := range diag.Required {
		if _, err := os.Stat(filepath.Join(a.BinDir(), f)); err != nil {
			return fmt.Errorf("bin/%s is missing; antivirus may have quarantined it (see README)", f)
		}
	}
	return nil
}

func quote(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}
