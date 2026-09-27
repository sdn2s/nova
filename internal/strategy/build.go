package strategy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	GameOff = "off"
	GameTCP = "tcp"
	GameUDP = "udp"
	GameAll = "all"
)

const (
	IpsetNone   = "none"
	IpsetAny    = "any"
	IpsetLoaded = "loaded"
)

const IpsetAllName = "ipset-all.txt"

type Env struct {
	BinDir   string
	ListsDir string

	Game    string
	GameTCP string
	GameUDP string

	Ipset string

	ListUsable func(path string) bool
}

type Result struct {
	Args  []string
	Notes []string
}

func Build(s *Strategy, env Env) (*Result, error) {
	usable := env.ListUsable
	if usable == nil {
		usable = ListFileUsable
	}
	gameTCP := env.Game == GameTCP || env.Game == GameAll
	gameUDP := env.Game == GameUDP || env.Game == GameAll
	if gameTCP && env.GameTCP == "" || gameUDP && env.GameUDP == "" {
		return nil, fmt.Errorf("game filter %q needs port ranges", env.Game)
	}

	res := &Result{}
	var profiles [][]string
	var tcpPorts, udpPorts []string

	for i, p := range s.Profiles {
		label := p.Name
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		tcp, udp := p.TCP, p.UDP
		switch p.Game {
		case GameTCP:
			if !gameTCP {
				res.Notes = append(res.Notes, fmt.Sprintf("profile %s skipped: game filter tcp is off", label))
				continue
			}
			tcp = env.GameTCP
		case GameUDP:
			if !gameUDP {
				res.Notes = append(res.Notes, fmt.Sprintf("profile %s skipped: game filter udp is off", label))
				continue
			}
			udp = env.GameUDP
		}

		lists := &listFilter{env: env, usable: usable, label: label}
		hostlist := lists.keep("hostlist", p.Hostlist, true)
		ipset := lists.keep("ipset", p.Ipset, true)
		hostlistEx := lists.keep("hostlist-exclude", p.HostlistExclude, false)
		ipsetEx := lists.keep("ipset-exclude", p.IpsetExclude, false)
		res.Notes = append(res.Notes, lists.notes...)
		if lists.drop != "" {
			res.Notes = append(res.Notes, fmt.Sprintf("profile %s skipped: %s", label, lists.drop))
			continue
		}

		var a []string
		if tcp != "" {
			a = append(a, "--filter-tcp="+tcp)
			tcpPorts = appendPorts(tcpPorts, tcp)
		}
		if udp != "" {
			a = append(a, "--filter-udp="+udp)
			udpPorts = appendPorts(udpPorts, udp)
		}
		if p.L3 != "" {
			a = append(a, "--filter-l3="+p.L3)
		}
		if p.L7 != "" {
			a = append(a, "--filter-l7="+p.L7)
		}
		a = append(a, hostlist...)
		a = append(a, ipset...)
		if len(p.Domains) > 0 {
			a = append(a, "--hostlist-domains="+strings.Join(p.Domains, ","))
		}
		a = append(a, hostlistEx...)
		a = append(a, ipsetEx...)
		if len(p.ExcludeDomains) > 0 {
			a = append(a, "--hostlist-exclude-domains="+strings.Join(p.ExcludeDomains, ","))
		}
		for _, arg := range p.Args {
			a = append(a, expandBin(arg, env.BinDir))
		}
		profiles = append(profiles, a)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("strategy %s: every profile was skipped", s.Name)
	}

	wfTCP, wfUDP := strings.Join(tcpPorts, ","), strings.Join(udpPorts, ",")
	if s.WFTCP != "" {
		wfTCP = s.WFTCP
		if gameTCP {
			wfTCP += "," + env.GameTCP
		}
	}
	if s.WFUDP != "" {
		wfUDP = s.WFUDP
		if gameUDP {
			wfUDP += "," + env.GameUDP
		}
	}
	if wfTCP != "" {
		res.Args = append(res.Args, "--wf-tcp="+wfTCP)
	}
	if wfUDP != "" {
		res.Args = append(res.Args, "--wf-udp="+wfUDP)
	}
	for i, a := range profiles {
		if i > 0 {
			res.Args = append(res.Args, "--new")
		}
		res.Args = append(res.Args, a...)
	}
	return res, nil
}

type listFilter struct {
	env    Env
	usable func(string) bool
	label  string
	notes  []string
	drop   string
}

func (f *listFilter) keep(opt string, names []string, include bool) []string {
	var out []string
	lost := false
	for _, n := range names {
		if include && opt == "ipset" && n == IpsetAllName {
			switch f.env.Ipset {
			case IpsetAny:

				continue
			case IpsetNone, "":
				lost = true
				continue
			}
		}
		path := f.env.ListsDir + string(filepath.Separator) + n
		if !f.usable(path) {

			if !strings.HasSuffix(n, "-user.txt") {
				f.notes = append(f.notes, fmt.Sprintf("profile %s: %s %s is missing or empty, omitted", f.label, opt, n))
			}
			lost = true
			continue
		}
		out = append(out, "--"+opt+"="+path)
	}
	if include && lost && len(out) == 0 && f.drop == "" {
		f.drop = opt + " lists are empty"
		if opt == "ipset" && f.env.Ipset != IpsetLoaded {
			f.drop = "ipset mode is " + orNone(f.env.Ipset)
		}
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return IpsetNone
	}
	return s
}

func expandBin(arg, binDir string) string {
	return strings.ReplaceAll(arg, "{bin}/", binDir+string(filepath.Separator))
}

func appendPorts(list []string, ports string) []string {
	for _, p := range strings.Split(ports, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dup := false
		for _, q := range list {
			if q == p {
				dup = true
				break
			}
		}
		if !dup {
			list = append(list, p)
		}
	}
	return list
}

func ListFileUsable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}
	return false
}
