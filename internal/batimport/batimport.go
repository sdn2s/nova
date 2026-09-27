package batimport

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"nova/internal/strategy"
)

const (
	VarBin     = "%BIN%"
	VarLists   = "%LISTS%"
	VarGameTCP = "%GameFilterTCP%"
	VarGameUDP = "%GameFilterUDP%"
)

func Command(src string) ([]string, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	var joined []string
	var cur strings.Builder
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimRight(line, " \t")
		if strings.HasSuffix(t, "^") && !strings.HasSuffix(t, "^^") {
			cur.WriteString(strings.TrimSuffix(t, "^"))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(t)
		joined = append(joined, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		joined = append(joined, cur.String())
	}

	var cmd string
	for _, l := range joined {
		if strings.Contains(strings.ToLower(l), "winws.exe") {
			if cmd != "" {
				return nil, fmt.Errorf("more than one winws.exe command")
			}
			cmd = l
		}
	}
	if cmd == "" {
		return nil, fmt.Errorf("no winws.exe command")
	}

	toks := tokenize(cmd)
	for i, t := range toks {
		if strings.HasSuffix(strings.ToLower(t), "winws.exe") {
			return toks[i+1:], nil
		}
	}
	return nil, fmt.Errorf("winws.exe token not found")
}

func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	inQ, have := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQ = !inQ
			have = true
		case c == '^' && !inQ && i+1 < len(s):
			i++
			b.WriteByte(s[i])
			have = true
		case (c == ' ' || c == '\t') && !inQ:
			if have {
				out = append(out, b.String())
				b.Reset()
				have = false
			}
		default:
			b.WriteByte(c)
			have = true
		}
	}
	if have {
		out = append(out, b.String())
	}
	return out
}

func Convert(name string, args []string) (*strategy.Strategy, error) {
	s := &strategy.Strategy{Name: name}
	var wfTCP, wfUDP string
	p := strategy.Profile{}
	flush := func() {
		p.Name = profileName(p)
		s.Profiles = append(s.Profiles, p)
		p = strategy.Profile{}
	}
	for _, a := range args {
		if a == "--new" {
			flush()
			continue
		}
		if !strings.HasPrefix(a, "--") {
			return nil, fmt.Errorf("unexpected token %q", a)
		}
		key, val, _ := strings.Cut(a, "=")
		switch key {
		case "--wf-tcp":
			wfTCP = val
		case "--wf-udp":
			wfUDP = val
		case "--filter-tcp":
			if val == VarGameTCP {
				p.Game = strategy.GameTCP
			} else if err := noGame(val); err != nil {
				return nil, err
			} else {
				p.TCP = val
			}
		case "--filter-udp":
			if val == VarGameUDP {
				p.Game = strategy.GameUDP
			} else if err := noGame(val); err != nil {
				return nil, err
			} else {
				p.UDP = val
			}
		case "--filter-l3":
			p.L3 = val
		case "--filter-l7":
			p.L7 = val
		case "--hostlist", "--hostlist-exclude", "--ipset", "--ipset-exclude":
			if !strings.HasPrefix(val, VarLists) {
				return nil, fmt.Errorf("%s: expected %s path, got %q", key, VarLists, val)
			}
			n := strings.TrimPrefix(val, VarLists)
			switch key {
			case "--hostlist":
				p.Hostlist = append(p.Hostlist, n)
			case "--hostlist-exclude":
				p.HostlistExclude = append(p.HostlistExclude, n)
			case "--ipset":
				p.Ipset = append(p.Ipset, n)
			case "--ipset-exclude":
				p.IpsetExclude = append(p.IpsetExclude, n)
			}
		case "--hostlist-domains":
			p.Domains = append(p.Domains, strings.Split(val, ",")...)
		case "--hostlist-exclude-domains":
			p.ExcludeDomains = append(p.ExcludeDomains, strings.Split(val, ",")...)
		default:
			if strings.HasPrefix(key, "--wf-") {
				return nil, fmt.Errorf("unsupported global option %s", key)
			}
			if strings.Contains(a, "%") && !strings.Contains(a, VarBin) {
				return nil, fmt.Errorf("unknown variable in %q", a)
			}
			p.Args = append(p.Args, strings.ReplaceAll(a, VarBin, "{bin}/"))
		}
	}
	flush()

	var tcp, udp []string
	for _, pr := range s.Profiles {
		tcp = append(tcp, splitPorts(pr.TCP)...)
		udp = append(udp, splitPorts(pr.UDP)...)
	}
	if w := stripVar(wfTCP, VarGameTCP); !samePorts(w, tcp) {
		s.WFTCP = w
	}
	if w := stripVar(wfUDP, VarGameUDP); !samePorts(w, udp) {
		s.WFUDP = w
	}
	return s, s.Validate()
}

func noGame(v string) error {
	if strings.Contains(v, "%Game") {
		return fmt.Errorf("game filter mixed with other ports: %q", v)
	}
	return nil
}

func stripVar(v, variable string) string {
	var out []string
	for _, p := range splitPorts(v) {
		if p != variable {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

func splitPorts(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func samePorts(declared string, computed []string) bool {
	a := uniqSorted(splitPorts(declared))
	b := uniqSorted(computed)
	return strings.Join(a, ",") == strings.Join(b, ",")
}

func uniqSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		m[v] = true
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func profileName(p strategy.Profile) string {
	var parts []string
	switch {
	case p.Game != "":
		parts = append(parts, "game-"+p.Game)
	case p.TCP != "" && p.UDP != "":
		parts = append(parts, "tcp:"+p.TCP, "udp:"+p.UDP)
	case p.TCP != "":
		parts = append(parts, "tcp:"+p.TCP)
	case p.UDP != "":
		parts = append(parts, "udp:"+p.UDP)
	}
	switch {
	case p.L7 != "":
		parts = append(parts, p.L7)
	case len(p.Hostlist) > 0:
		parts = append(parts, strings.TrimSuffix(p.Hostlist[0], ".txt"))
	case len(p.Ipset) > 0:
		parts = append(parts, strings.TrimSuffix(p.Ipset[0], ".txt"))
	case len(p.Domains) > 0:
		parts = append(parts, p.Domains[0])
	}
	return strings.Join(parts, " ")
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func NameFromFile(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if i, j := strings.Index(base, "("), strings.LastIndex(base, ")"); i >= 0 && j > i {
		base = base[i+1 : j]
	}
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(base), "-"), "-")
}
