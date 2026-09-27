package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Strategy struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Source      string `toml:"source,omitempty"`

	WFTCP string `toml:"wf_tcp,omitempty"`
	WFUDP string `toml:"wf_udp,omitempty"`

	Profiles []Profile `toml:"profile"`
}

type Profile struct {
	Name string `toml:"name,omitempty"`

	TCP string `toml:"tcp,omitempty"`
	UDP string `toml:"udp,omitempty"`
	L3  string `toml:"l3,omitempty"`
	L7  string `toml:"l7,omitempty"`

	Game string `toml:"game,omitempty"`

	Hostlist        []string `toml:"hostlist,omitempty"`
	HostlistExclude []string `toml:"hostlist_exclude,omitempty"`
	Ipset           []string `toml:"ipset,omitempty"`
	IpsetExclude    []string `toml:"ipset_exclude,omitempty"`

	Domains        []string `toml:"domains,omitempty"`
	ExcludeDomains []string `toml:"exclude_domains,omitempty"`

	Args []string `toml:"args,omitempty"`
}

func Load(path string) (*Strategy, error) {
	var s Strategy
	if _, err := toml.DecodeFile(path, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.Name == "" {
		s.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

func LoadDir(dir string) ([]*Strategy, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.toml"))
	if err != nil {
		return nil, err
	}
	var out []*Strategy
	for _, p := range paths {
		s, err := Load(p)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return naturalLess(out[i].Name, out[j].Name) })
	return out, nil
}

func (s *Strategy) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	enc.Indent = ""
	return enc.Encode(s)
}

func (s *Strategy) Validate() error {
	if len(s.Profiles) == 0 {
		return fmt.Errorf("no profiles")
	}
	for i, p := range s.Profiles {
		switch p.Game {
		case "":
		case "tcp":
			if p.TCP != "" || p.UDP != "" {
				return fmt.Errorf("profile %d: game profile must not set tcp/udp", i+1)
			}
		case "udp":
			if p.TCP != "" || p.UDP != "" {
				return fmt.Errorf("profile %d: game profile must not set tcp/udp", i+1)
			}
		default:
			return fmt.Errorf("profile %d: game must be tcp or udp, got %q", i+1, p.Game)
		}
		if p.TCP == "" && p.UDP == "" && p.Game == "" && p.L7 == "" {
			return fmt.Errorf("profile %d: no tcp/udp/l7 filter", i+1)
		}
		for _, a := range p.Args {
			if !strings.HasPrefix(a, "--") {
				return fmt.Errorf("profile %d: arg %q must start with --", i+1, a)
			}
			if a == "--new" {
				return fmt.Errorf("profile %d: --new is not allowed in args", i+1)
			}
		}
	}
	return nil
}

func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digitPrefix(a), digitPrefix(b)
		if da != "" && db != "" {
			if len(da) != len(db) {
				return len(da) < len(db)
			}
			if da != db {
				return da < db
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digitPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}
