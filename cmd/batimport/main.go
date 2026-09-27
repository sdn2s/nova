package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nova/internal/batimport"
)

func main() {
	src := flag.String("src", "", "flowseal/zapret-discord-youtube checkout")
	out := flag.String("out", "strategies", "output directory")
	flag.Parse()
	if *src == "" {
		fmt.Fprintln(os.Stderr, "usage: batimport -src DIR [-out strategies]")
		os.Exit(2)
	}
	if err := run(*src, *out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(src, out string) error {
	ver, _ := os.ReadFile(filepath.Join(src, ".service", "version.txt"))
	files, err := filepath.Glob(filepath.Join(src, "general*.bat"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no general*.bat in %s", src)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		args, err := batimport.Command(string(b))
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		name := batimport.NameFromFile(f)
		s, err := batimport.Convert(name, args)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		s.Source = fmt.Sprintf("flowseal/zapret-discord-youtube %s, %s", strings.TrimSpace(string(ver)), filepath.Base(f))
		if err := s.Save(filepath.Join(out, name+".toml")); err != nil {
			return err
		}
		fmt.Printf("%-22s %d profiles\n", name, len(s.Profiles))
	}
	return nil
}
