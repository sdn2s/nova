package probe

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/BurntSushi/toml"
)

type Target struct {
	Name string `toml:"name"`
	URL  string `toml:"url"`
}

//go:embed targets.toml
var defaultTargets []byte

func LoadTargets(path string) ([]Target, error) {
	var f struct {
		Target []Target `toml:"target"`
	}
	var err error
	if path != "" {
		_, err = toml.DecodeFile(path, &f)
	}
	if path == "" || errors.Is(err, fs.ErrNotExist) {
		_, err = toml.Decode(string(defaultTargets), &f)
	}
	if err != nil {
		return nil, err
	}
	if len(f.Target) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	return f.Target, nil
}

type FreezeHost struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Host     string `json:"host"`
}

const SuiteURL = "https://hyperion-cs.github.io/dpi-checkers/ru/tcp-16-20/suite.v2.json"

//go:embed suite.json
var suiteFallback []byte

func FreezeSuite(ctx context.Context) ([]FreezeHost, bool) {
	data, live := fetchSuite(ctx), true
	if data == nil {
		data, live = suiteFallback, false
	}
	var all []FreezeHost
	if err := json.Unmarshal(data, &all); err != nil {
		json.Unmarshal(suiteFallback, &all)
		live = false
	}
	seen := map[string]bool{}
	var out []FreezeHost
	for _, h := range all {
		if h.Host == "" || seen[h.Provider] {
			continue
		}
		seen[h.Provider] = true
		out = append(out, h)
	}
	return out, live
}

func fetchSuite(ctx context.Context) []byte {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, SuiteURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var raw json.RawMessage
	if json.NewDecoder(resp.Body).Decode(&raw) != nil {
		return nil
	}
	return raw
}
