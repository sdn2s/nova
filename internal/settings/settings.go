package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"

	"github.com/BurntSushi/toml"

	"nova/internal/strategy"
)

type Settings struct {
	Strategy   string `toml:"strategy"`
	GameFilter string `toml:"game_filter"`
	GameTCP    string `toml:"game_tcp"`
	GameUDP    string `toml:"game_udp"`
	Ipset      string `toml:"ipset"`
}

func Default() Settings {
	return Settings{
		Strategy:   "general",
		GameFilter: strategy.GameOff,
		GameTCP:    "1024-65535",
		GameUDP:    "1024-65535",
		Ipset:      strategy.IpsetNone,
	}
}

func Load(path string) (Settings, error) {
	s := Default()
	if _, err := toml.DecodeFile(path, &s); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, s.Validate()
}

func (s Settings) Save(path string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(s)
}

var portsRe = regexp.MustCompile(`^\d{1,5}(-\d{1,5})?(,\d{1,5}(-\d{1,5})?)*$`)

func (s Settings) Validate() error {
	switch s.GameFilter {
	case strategy.GameOff, strategy.GameTCP, strategy.GameUDP, strategy.GameAll:
	default:
		return fmt.Errorf("game_filter must be off, tcp, udp or all, got %q", s.GameFilter)
	}
	switch s.Ipset {
	case strategy.IpsetNone, strategy.IpsetAny, strategy.IpsetLoaded:
	default:
		return fmt.Errorf("ipset must be none, any or loaded, got %q", s.Ipset)
	}
	for k, v := range map[string]string{"game_tcp": s.GameTCP, "game_udp": s.GameUDP} {
		if !portsRe.MatchString(v) {
			return fmt.Errorf("%s: bad port list %q", k, v)
		}
	}
	return nil
}

func (s *Settings) Set(key, value string) error {
	switch key {
	case "strategy":
		s.Strategy = value
	case "game_filter":
		s.GameFilter = value
	case "game_tcp":
		s.GameTCP = value
	case "game_udp":
		s.GameUDP = value
	case "ipset":
		s.Ipset = value
	default:
		return fmt.Errorf("unknown key %q (strategy, game_filter, game_tcp, game_udp, ipset)", key)
	}
	return s.Validate()
}
