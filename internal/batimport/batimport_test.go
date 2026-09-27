package batimport

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"nova/internal/strategy"
)

const gameRange = "1024-65535"

func TestParity(t *testing.T) {
	files, _ := filepath.Glob("testdata/upstream/*.bat")
	if len(files) == 0 {
		t.Fatal("no testdata")
	}
	sep := string(filepath.Separator)
	for _, f := range files {
		t.Run(NameFromFile(f), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := Command(string(src))
			if err != nil {
				t.Fatal(err)
			}
			s, err := Convert(NameFromFile(f), raw)
			if err != nil {
				t.Fatal(err)
			}
			res, err := strategy.Build(s, strategy.Env{
				BinDir: "/B", ListsDir: "/L",
				Game: strategy.GameAll, GameTCP: gameRange, GameUDP: gameRange,
				Ipset:      strategy.IpsetLoaded,
				ListUsable: func(string) bool { return true },
			})
			if err != nil {
				t.Fatal(err)
			}

			r := strings.NewReplacer(VarBin, "/B"+sep, VarLists, "/L"+sep, VarGameTCP, gameRange, VarGameUDP, gameRange)
			var want []string
			for _, a := range raw {
				want = append(want, r.Replace(a))
			}

			gw, gs := normalize(res.Args), normalize(want)
			if len(gw) != len(gs) {
				t.Fatalf("profiles: got %d, want %d", len(gw), len(gs))
			}
			for i := range gs {
				if gw[i] != gs[i] {
					t.Errorf("segment %d differs\n got: %s\nwant: %s", i, gw[i], gs[i])
				}
			}
		})
	}
}

func normalize(args []string) []string {
	var wf []string
	var segs [][]string
	cur := []string{}
	for _, a := range args {
		switch {
		case a == "--new":
			segs = append(segs, cur)
			cur = []string{}
		case strings.HasPrefix(a, "--wf-"):
			k, v, _ := strings.Cut(a, "=")
			wf = append(wf, k+"="+strings.Join(uniqSorted(splitPorts(v)), ","))
		default:
			cur = append(cur, a)
		}
	}
	segs = append(segs, cur)
	sort.Strings(wf)
	out := []string{strings.Join(wf, " ")}
	for _, s := range segs {
		sort.Strings(s)
		out = append(out, strings.Join(s, " "))
	}
	return out
}

func TestCommandEscapes(t *testing.T) {
	src := "@echo off\r\nstart \"x\" /min \"%BIN%winws.exe\" --wf-tcp=443 ^\r\n--filter-tcp=443 --dpi-desync-fake-tls=^! --hostlist=\"%LISTS%a b.txt\"\r\n"
	got, err := Command(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--wf-tcp=443", "--filter-tcp=443", "--dpi-desync-fake-tls=!", "--hostlist=%LISTS%a b.txt"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
}

func TestNameFromFile(t *testing.T) {
	for in, want := range map[string]string{
		"general.bat":                      "general",
		"general (ALT10).bat":              "alt10",
		"general (FAKE TLS AUTO ALT2).bat": "fake-tls-auto-alt2",
	} {
		if got := NameFromFile(in); got != want {
			t.Errorf("%s: got %s, want %s", in, got, want)
		}
	}
}
