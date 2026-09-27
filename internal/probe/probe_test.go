package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okServer(t *testing.T) *httptest.Server {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func checker(srv *httptest.Server) Checker {
	return Checker{Timeout: time.Second, RootCAs: srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
}

func freezeServer(t *testing.T, cert tls.Certificate) string {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				io.CopyN(io.Discard, c, 16*1024)
				time.Sleep(3 * time.Second)
				c.Close()
			}(c)
		}
	}()
	return ln.Addr().String()
}

func TestChecks(t *testing.T) {
	srv := okServer(t)
	c := checker(srv)
	ctx := context.Background()
	host := strings.TrimPrefix(srv.URL, "https://")

	for _, test := range Tests {
		if r := c.URL(ctx, "ok", srv.URL, test); r.Kind != KindOK || r.Code != 403 {
			t.Errorf("%s: %+v", test.Name, r)
		}
	}
	if r := (&Checker{Timeout: time.Second}).URL(ctx, "ssl", srv.URL, Tests[0]); r.Kind != KindSSL {
		t.Errorf("untrusted cert: %+v", r)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	ln.Close()
	if r := c.URL(ctx, "closed", "https://"+closed, Tests[0]); r.Kind != KindFail {
		t.Errorf("closed port: %+v", r)
	}
	if r := c.Freeze(ctx, "ok", host); r.Kind != KindOK {
		t.Errorf("freeze on healthy server: %+v", r)
	}
	if r := (&Checker{Timeout: time.Second}).Freeze(ctx, "untrusted", host); r.Kind != KindOK {
		t.Errorf("freeze must ignore certificates: %+v", r)
	}
	tls12only := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tls12only.TLS = &tls.Config{MaxVersion: tls.VersionTLS12}
	tls12only.StartTLS()
	defer tls12only.Close()
	c12 := checker(tls12only)
	if r := c12.URL(ctx, "old", tls12only.URL, Tests[2]); r.Kind != KindUnsup {
		t.Errorf("tls13 against tls12-only server: %+v", r)
	}
	frozen := freezeServer(t, srv.TLS.Certificates[0])
	if r := c.Freeze(ctx, "frozen", frozen); r.Kind != KindBlocked {
		t.Errorf("freeze: %+v", r)
	}
}

func TestProber(t *testing.T) {
	srv := okServer(t)
	started := map[string]bool{}
	stopped := map[string]bool{}
	p := &Prober{
		Checker:  checker(srv),
		Targets:  []Target{{"a", srv.URL}, {"b", srv.URL}},
		Parallel: 4,
		Start: func(ctx context.Context, name string, args []string) (func(), error) {
			if name == "broken" {
				return nil, errors.New("exited at start")
			}
			started[name] = true
			return func() { stopped[name] = true }, nil
		},
	}
	good := p.Strategy(context.Background(), Candidate{Name: "good"})
	if good.OK != 2*len(Tests) || good.Total() != good.OK || !started["good"] || !stopped["good"] {
		t.Fatalf("good: %+v", good)
	}
	broken := p.Strategy(context.Background(), Candidate{Name: "broken"})
	if broken.Err == "" || broken.Total() != 0 {
		t.Fatalf("broken: %+v", broken)
	}
}

func TestRank(t *testing.T) {
	got := Rank([]Score{
		{Name: "crashed", Err: "x"},
		{Name: "few", OK: 3},
		{Name: "many-frozen", OK: 9, Blocked: 2},
		{Name: "many", OK: 9},
		{Name: "many-later", OK: 9},
	})
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}
	if want := "many many-later many-frozen few crashed"; strings.Join(names, " ") != want {
		t.Fatalf("got %v, want %s", names, want)
	}
}

func TestTargets(t *testing.T) {
	ts, err := LoadTargets("/nonexistent/targets.toml")
	if err != nil || len(ts) != 12 {
		t.Fatalf("default targets: %d %v", len(ts), err)
	}
	var all []FreezeHost
	hosts, _ := FreezeSuite(canceled())
	seen := map[string]bool{}
	for _, h := range hosts {
		if seen[h.Provider] {
			t.Fatalf("provider %s twice", h.Provider)
		}
		seen[h.Provider] = true
		all = append(all, h)
	}
	if len(all) < 10 {
		t.Fatalf("fallback suite: %d hosts", len(all))
	}
}

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
