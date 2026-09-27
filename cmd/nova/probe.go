package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"nova/internal/app"
	"nova/internal/probe"
	"nova/internal/service"
	"nova/internal/strategy"
	"nova/internal/winsys"
	"nova/internal/winws"
)

func cmdProbe(a *app.App, argv []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	only := fs.String("only", "", "comma-separated strategies to test (default: all)")
	freeze := fs.Bool("freeze", false, "also check the TCP 16-20 KB block on hosting providers")
	freezeHost := fs.String("freeze-host", "", "extra host for the freeze check, e.g. your own VPS")
	timeout := fs.Duration("timeout", 5*time.Second, "per-check timeout")
	parallel := fs.Int("parallel", 16, "concurrent checks")
	keepIpset := fs.Bool("keep-ipset", false, "test with the configured ipset mode instead of 'any'")
	save := fs.Bool("save", false, "save the best strategy to settings")
	install := fs.Bool("install", false, "save the best strategy and install the service with it")
	if err := fs.Parse(argv); err != nil {
		return err
	}

	cands, err := candidates(a, *only, *keepIpset)
	if err != nil {
		return err
	}
	if err := checkInstall(a); err != nil {
		return err
	}
	if err := winws.Prepare(); err != nil {
		return err
	}

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	wasRunning, err := service.Stop(service.Name)
	if err != nil {
		return fmt.Errorf("stop service %s: %w", service.Name, err)
	}
	installed := false
	defer func() {
		if wasRunning && !installed {
			if err := service.Start(service.Name); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not restart service %s: %v\n", service.Name, err)
			} else {
				fmt.Printf("service %s restarted with its previous strategy\n", service.Name)
			}
		}
	}()
	if procs, err := winsys.Processes(); err == nil && procs["winws.exe"] > 0 {
		return fmt.Errorf("another winws.exe is running (flowseal zapret?); stop it first, see: nova diag")
	}

	targets, err := probe.LoadTargets(filepath.Join(a.Home, "targets.toml"))
	if err != nil {
		return err
	}
	var hosts []probe.FreezeHost
	if *freeze {
		var live bool
		hosts, live = probe.FreezeSuite(ctx)
		if !live {
			fmt.Println("freeze suite: download failed, using the built-in copy")
		}
	}
	if *freezeHost != "" {
		hosts = append(hosts, probe.FreezeHost{ID: "custom", Provider: *freezeHost, Host: *freezeHost})
	}

	logDir := filepath.Join(a.Home, "logs")
	p := &probe.Prober{
		Checker:  probe.Checker{Timeout: *timeout},
		Targets:  targets,
		Freeze:   hosts,
		Parallel: *parallel,
		Start: func(ctx context.Context, name string, args []string) (func(), error) {
			proc, err := winws.Start(a.WinwsPath(), args, filepath.Join(logDir, "probe-"+name+".log"), 1500*time.Millisecond)
			if err != nil {
				return nil, err
			}
			return proc.Stop, nil
		},
	}

	fmt.Printf("%d strategies, %d targets x %d tests", len(cands), len(targets), len(probe.Tests))
	if len(hosts) > 0 {
		fmt.Printf(" + %d freeze hosts", len(hosts))
	}
	fmt.Println()
	base := p.Baseline(ctx)
	printScore(os.Stdout, "", base)

	var scores []probe.Score
	for i, c := range cands {
		if ctx.Err() != nil {
			fmt.Println("interrupted")
			break
		}
		s := p.Strategy(ctx, c)
		scores = append(scores, s)
		printScore(os.Stdout, fmt.Sprintf("[%d/%d] ", i+1, len(cands)), s)
	}
	if len(scores) == 0 {
		return nil
	}

	ranked := probe.Rank(scores)
	best := ranked[0]
	report := filepath.Join(logDir, "probe-"+time.Now().Format("20060102-150405")+".txt")
	if err := writeReport(report, base, ranked); err != nil {
		fmt.Fprintln(os.Stderr, "warning: report:", err)
	}

	fmt.Println("\nranking:")
	for i, s := range ranked {
		if i == 5 {
			fmt.Printf("  ... full report: %s\n", report)
			break
		}
		printScore(os.Stdout, "  ", s)
	}
	if ctx.Err() != nil {
		fmt.Println("\ninterrupted: nothing saved")
		return nil
	}
	if base.Total() > 0 && base.OK == base.Total() {
		fmt.Println("\nevery target is reachable without bypass: nothing to fix here")
		return nil
	}
	if best.Err != "" || best.OK <= base.OK {
		fmt.Println("\nno strategy improved on the baseline; see nova diag")
		return nil
	}
	fmt.Printf("\nbest: %s (%d/%d ok, without bypass %d/%d)\n", best.Name, best.OK, best.Total(), base.OK, base.Total())
	if fails := failures(best); fails != "" {
		fmt.Println("still failing:", fails)
	}

	if !*save && !*install {
		fmt.Printf("apply: nova probe --install (or nova service install %s)\n", best.Name)
		return nil
	}
	a.Settings.Strategy = best.Name
	if err := a.SaveSettings(); err != nil {
		return err
	}
	fmt.Println("saved to settings")
	if *install {
		_, r, err := a.Build(best.Name)
		if err != nil {
			return err
		}
		if err := service.Install(a.WinwsPath(), r.Args); err != nil {
			return err
		}
		installed = true
		fmt.Printf("service %s running with strategy %s\n", service.Name, best.Name)
	}
	return nil
}

func candidates(a *app.App, only string, keepIpset bool) ([]probe.Candidate, error) {
	list, err := a.Strategies()
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range strings.Split(only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	env := a.Env()
	if !keepIpset {
		env.Ipset = strategy.IpsetAny
	}
	var out []probe.Candidate
	for _, s := range list {
		if len(want) > 0 && !want[s.Name] {
			continue
		}
		delete(want, s.Name)
		r, err := strategy.Build(s, env)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", s.Name, err)
			continue
		}
		out = append(out, probe.Candidate{Name: s.Name, Args: r.Args})
	}
	for n := range want {
		return nil, fmt.Errorf("strategy %q not found", n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no strategies to test")
	}
	return out, nil
}

func printScore(w io.Writer, prefix string, s probe.Score) {
	if s.Err != "" {
		fmt.Fprintf(w, "%s%-22s did not start: %s\n", prefix, s.Name, s.Err)
		return
	}
	fmt.Fprintf(w, "%s%-22s ok %3d/%-3d fail %3d  ssl %2d", prefix, s.Name, s.OK, s.Total(), s.Fail, s.SSL)
	if s.Unsup > 0 {
		fmt.Fprintf(w, "  unsup %d", s.Unsup)
	}
	if s.Blocked > 0 {
		fmt.Fprintf(w, "  frozen %d", s.Blocked)
	}
	fmt.Fprintln(w)
}

func failures(s probe.Score) string {
	var order []string
	by := map[string][]string{}
	for _, r := range s.Results {
		if r.Kind == probe.KindOK {
			continue
		}
		if _, ok := by[r.Target]; !ok {
			order = append(order, r.Target)
		}
		by[r.Target] = append(by[r.Target], r.Test)
	}
	var parts []string
	for _, t := range order {
		parts = append(parts, t+"("+strings.Join(by[t], ",")+")")
	}
	return strings.Join(parts, " ")
}

func writeReport(path string, base probe.Score, ranked []probe.Score) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	printScore(f, "", base)
	for _, s := range ranked {
		printScore(f, "", s)
	}
	for _, s := range append([]probe.Score{base}, ranked...) {
		fmt.Fprintf(f, "\n== %s\n", s.Name)
		for _, r := range s.Results {
			fmt.Fprintf(f, "%-22s %-6s %-7s %3d %s\n", r.Target, r.Test, r.Kind, r.Code, r.Err)
		}
	}
	return nil
}
