package strategy

import (
	"strings"
	"testing"
)

func testStrategy() *Strategy {
	return &Strategy{Name: "t", Profiles: []Profile{
		{Name: "general", TCP: "80,443", Hostlist: []string{"list-general.txt", "list-general-user.txt"},
			HostlistExclude: []string{"list-exclude-user.txt"}, Args: []string{"--dpi-desync=fake", "--dpi-desync-fake-tls={bin}/tls.bin"}},
		{Name: "ipset", TCP: "443,8443", Ipset: []string{IpsetAllName}, Args: []string{"--dpi-desync=multisplit"}},
		{Name: "game", Game: GameUDP, Ipset: []string{IpsetAllName}, Args: []string{"--dpi-desync=fake"}},
	}}
}

func only(names ...string) func(string) bool {
	return func(p string) bool {
		for _, n := range names {
			if strings.HasSuffix(p, "/"+n) {
				return true
			}
		}
		return false
	}
}

func build(t *testing.T, env Env) string {
	t.Helper()
	env.BinDir, env.ListsDir = "/B", "/L"
	r, err := Build(testStrategy(), env)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(r.Args, " ")
}

func TestIpsetNoneDropsProfilesAndGameOff(t *testing.T) {
	got := build(t, Env{Game: GameOff, Ipset: IpsetNone, ListUsable: only("list-general.txt", "list-general-user.txt", IpsetAllName)})
	want := "--wf-tcp=80,443 --filter-tcp=80,443 --hostlist=/L/list-general.txt --hostlist=/L/list-general-user.txt --dpi-desync=fake --dpi-desync-fake-tls=/B/tls.bin"
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestIpsetAnyRemovesRestriction(t *testing.T) {
	got := build(t, Env{Game: GameUDP, GameUDP: "1024-65535", Ipset: IpsetAny, ListUsable: only("list-general.txt")})
	want := "--wf-tcp=80,443,8443 --wf-udp=1024-65535 " +
		"--filter-tcp=80,443 --hostlist=/L/list-general.txt --dpi-desync=fake --dpi-desync-fake-tls=/B/tls.bin " +
		"--new --filter-tcp=443,8443 --dpi-desync=multisplit " +
		"--new --filter-udp=1024-65535 --dpi-desync=fake"
	if got != want {
		t.Fatalf("\n got: %s\nwant: %s", got, want)
	}
}

func TestEmptyIncludeListsDropProfile(t *testing.T) {
	r, err := Build(testStrategy(), Env{BinDir: "/B", ListsDir: "/L", Game: GameOff, Ipset: IpsetLoaded, ListUsable: only(IpsetAllName)})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(r.Args, " ")
	if strings.Contains(got, "hostlist") || !strings.HasPrefix(got, "--wf-tcp=443,8443 --filter-tcp=443,8443 --ipset=/L/ipset-all.txt") {
		t.Fatalf("got: %s", got)
	}
}

func TestAllSkippedIsError(t *testing.T) {
	_, err := Build(testStrategy(), Env{Game: GameOff, Ipset: IpsetNone, ListUsable: only()})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestNaturalLess(t *testing.T) {
	if !naturalLess("alt2", "alt10") || naturalLess("alt10", "alt2") || !naturalLess("alt", "alt2") {
		t.Fatal("wrong order")
	}
}
