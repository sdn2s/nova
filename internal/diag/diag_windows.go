package diag

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"

	"nova/internal/service"
	"nova/internal/winsys"
)

type conflict struct {
	name  string
	match func(s winsys.Service) bool
	level Level
	hint  string
}

func has(s winsys.Service, words ...string) bool {
	hay := strings.ToLower(s.Name + " " + s.Display)
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

var conflicts = []conflict{
	{"Killer Networking", func(s winsys.Service) bool { return has(s, "killer") }, Fail,
		"Killer network services conflict with WinDivert; uninstall Killer Control Center"},
	{"Intel Connectivity Network Service", func(s winsys.Service) bool { return has(s, "intel", "connectivity", "network") }, Fail,
		"disable Intel Connectivity Network Service in services.msc"},
	{"Check Point", func(s winsys.Service) bool { return has(s, "tracsrvwrapper") || has(s, "epwd") }, Fail,
		"uninstall Check Point"},
	{"SmartByte", func(s winsys.Service) bool { return has(s, "smartbyte") }, Fail,
		"uninstall or disable SmartByte in services.msc"},
	{"VPN", func(s winsys.Service) bool { return has(s, "vpn") }, Warn,
		"some VPN clients conflict with WinDivert; disconnect VPNs if bypass does not work"},
}

func platformChecks(env Env) []Check {
	var out []Check
	if !winsys.IsElevated() {
		out = append(out, Check{Name: "admin rights", Level: Warn,
			Detail: "not elevated", Hint: "run as administrator for complete results and --fix"})
	}

	bfe := Check{Name: "Base Filtering Engine"}
	if st, err := service.State("BFE"); err != nil || st != "running" {
		bfe.Level = Fail
		bfe.Detail = "state: " + orUnknown(st, err)
		bfe.Hint = "WinDivert needs the BFE service; start it: sc config BFE start= auto && sc start BFE"
	} else {
		bfe.Detail = "running"
	}
	out = append(out, bfe)
	out = append(out, checkForeign())

	if svcs, err := winsys.ActiveServices(); err != nil {
		out = append(out, Check{Name: "services", Level: Warn, Detail: err.Error()})
	} else {
		for _, c := range conflicts {
			var found []string
			for _, s := range svcs {
				if c.match(s) {
					found = append(found, s.Display)
				}
			}
			ch := Check{Name: c.name, Detail: "not found"}
			if len(found) > 0 {
				ch.Level, ch.Detail, ch.Hint = c.level, strings.Join(found, ", "), c.hint
			}
			out = append(out, ch)
		}
	}

	procs, perr := winsys.Processes()
	if perr == nil {
		ag := Check{Name: "AdGuard", Detail: "not running"}
		if procs["adguardsvc.exe"] > 0 {
			ag.Level, ag.Detail = Fail, "AdguardSvc.exe is running"
			ag.Hint = "AdGuard's traffic filtering breaks Discord through winws; disable it or exclude Discord"
		}
		out = append(out, ag)
	}

	out = append(out, checkProxy(), checkDoH())
	if perr == nil {
		out = append(out, checkDriver(procs))
	}
	return out
}

func checkForeign() Check {
	c := Check{Name: "other bypass services", Detail: "none"}
	var found []string
	for _, n := range service.Foreign {
		if st, err := service.State(n); err == nil && st != "" {
			found = append(found, n+" ("+st+")")
		}
	}
	if len(found) == 0 {
		return c
	}
	c.Level = Fail
	c.Detail = strings.Join(found, ", ")
	c.Hint = "they also use WinDivert; nova diag --fix removes them"
	c.Fix = func() error {
		var errs []error
		for _, n := range service.Foreign {
			if err := service.Delete(n); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
	return c
}

func checkDriver(procs map[string]int) Check {
	c := Check{Name: "WinDivert driver"}
	var running []string
	for _, d := range service.DriverServices {
		if st, err := service.State(d); err == nil && st == "running" {
			running = append(running, d)
		}
	}
	switch {
	case len(running) == 0:
		c.Detail = "not loaded"
	case procs["winws.exe"] > 0:
		c.Detail = fmt.Sprintf("loaded (%s), winws.exe running", strings.Join(running, ", "))
	default:
		c.Level = Warn
		c.Detail = fmt.Sprintf("loaded (%s) but winws.exe is not running", strings.Join(running, ", "))
		c.Hint = "another program may hold the driver; nova diag --fix unloads it"
		c.Fix = func() error {
			var errs []error
			for _, d := range service.DriverServices {
				if err := service.Delete(d); err != nil {
					errs = append(errs, err)
				}
			}
			return errors.Join(errs...)
		}
	}
	return c
}

func checkProxy() Check {
	c := Check{Name: "system proxy", Detail: "disabled"}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return c
	}
	defer k.Close()
	if on, _, err := k.GetIntegerValue("ProxyEnable"); err == nil && on != 0 {
		srv, _, _ := k.GetStringValue("ProxyServer")
		c.Level, c.Detail = Warn, "enabled: "+srv
		c.Hint = "make sure the proxy works, or disable it if you do not use one"
	}
	return c
}

func checkDoH() Check {
	c := Check{Name: "encrypted DNS"}
	if dohEnabled(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Dnscache\InterfaceSpecificParameters`, 0) {
		c.Detail = "configured in Windows"
		return c
	}
	c.Level = Warn
	c.Detail = "not configured in Windows"
	c.Hint = "provider DNS may return fake addresses; enable secure DNS in the browser or in Windows 11 settings"
	return c
}

func dohEnabled(root registry.Key, path string, depth int) bool {
	if depth > 6 {
		return false
	}
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer k.Close()
	if v, _, err := k.GetIntegerValue("DohFlags"); err == nil && v > 0 {
		return true
	}
	subs, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}
	for _, s := range subs {
		if dohEnabled(root, path+`\`+s, depth+1) {
			return true
		}
	}
	return false
}

func orUnknown(s string, err error) string {
	if err != nil {
		return err.Error()
	}
	if s == "" {
		return "not installed"
	}
	return s
}
