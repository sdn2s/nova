package app

import (
	"fmt"
	"os"
	"path/filepath"

	"nova/internal/settings"
	"nova/internal/strategy"
)

type App struct {
	Home     string
	Settings settings.Settings
}

func Open(home string) (*App, error) {
	if home == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		if exe, err = filepath.EvalSymlinks(exe); err != nil {
			return nil, err
		}
		home = filepath.Dir(exe)
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	a := &App{Home: home}
	if a.Settings, err = settings.Load(a.SettingsPath()); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) SettingsPath() string  { return filepath.Join(a.Home, "nova.toml") }
func (a *App) BinDir() string        { return filepath.Join(a.Home, "bin") }
func (a *App) ListsDir() string      { return filepath.Join(a.Home, "lists") }
func (a *App) StrategiesDir() string { return filepath.Join(a.Home, "strategies") }
func (a *App) WinwsPath() string     { return filepath.Join(a.BinDir(), "winws.exe") }
func (a *App) SaveSettings() error   { return a.Settings.Save(a.SettingsPath()) }

func (a *App) Strategies() ([]*strategy.Strategy, error) {
	return strategy.LoadDir(a.StrategiesDir())
}

func (a *App) Strategy(name string) (*strategy.Strategy, error) {
	if name == "" {
		name = a.Settings.Strategy
	}
	p := filepath.Join(a.StrategiesDir(), name+".toml")
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("strategy %q not found in %s (see: nova list)", name, a.StrategiesDir())
	}
	return strategy.Load(p)
}

func (a *App) Env() strategy.Env {
	return strategy.Env{
		BinDir:   a.BinDir(),
		ListsDir: a.ListsDir(),
		Game:     a.Settings.GameFilter,
		GameTCP:  a.Settings.GameTCP,
		GameUDP:  a.Settings.GameUDP,
		Ipset:    a.Settings.Ipset,
	}
}

func (a *App) Build(name string) (*strategy.Strategy, *strategy.Result, error) {
	s, err := a.Strategy(name)
	if err != nil {
		return nil, nil, err
	}
	r, err := strategy.Build(s, a.Env())
	if err != nil {
		return nil, nil, err
	}
	return s, r, nil
}
