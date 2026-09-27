package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type Kind string

const (
	KindOK      Kind = "ok"
	KindFail    Kind = "fail"
	KindSSL     Kind = "ssl"
	KindBlocked Kind = "blocked"
	KindUnsup   Kind = "unsup"
)

type Test struct {
	Name       string
	MinVersion uint16
	MaxVersion uint16

	Insecure bool
}

var Tests = []Test{
	{Name: "http"},
	{Name: "tls12", MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12},
	{Name: "tls13", MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13},
}

var FreezeTest = Test{Name: "freeze", Insecure: true}

const FreezePayload = 64 * 1024

type Result struct {
	Target string
	Test   string
	Kind   Kind
	Code   int
	Err    string
}

type Checker struct {
	Timeout time.Duration
	RootCAs *x509.CertPool
}

func (c *Checker) client(t Test) *http.Client {
	tr := &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: min(3*time.Second, c.Timeout)}).DialContext,
		TLSHandshakeTimeout: c.Timeout,
		TLSClientConfig:     &tls.Config{MinVersion: t.MinVersion, MaxVersion: t.MaxVersion, RootCAs: c.RootCAs, InsecureSkipVerify: t.Insecure},
		TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{},
		DisableKeepAlives:   true,
	}
	return &http.Client{
		Transport:     tr,
		Timeout:       c.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (c *Checker) URL(ctx context.Context, name, url string, t Test) Result {
	r := Result{Target: name, Test: t.Name}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		r.Kind, r.Err = KindFail, err.Error()
		return r
	}
	resp, err := c.client(t).Do(req)
	if err != nil {
		r.Kind, r.Err = classify(err), err.Error()
		return r
	}
	resp.Body.Close()
	r.Kind, r.Code = KindOK, resp.StatusCode
	return r
}

func (c *Checker) Freeze(ctx context.Context, name, host string) Result {
	r := Result{Target: name, Test: FreezeTest.Name}
	payload := make([]byte, FreezePayload)
	rand.Read(payload)
	body := &countingReader{r: bytes.NewReader(payload)}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/", body)
	if err != nil {
		r.Kind, r.Err = KindFail, err.Error()
		return r
	}
	req.ContentLength = FreezePayload
	resp, err := c.client(FreezeTest).Do(req)
	if err != nil {
		r.Kind, r.Err = classify(err), err.Error()
		if r.Kind == KindFail && isTimeout(err) && body.n.Load() > 0 {
			r.Kind = KindBlocked
		}
		return r
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	r.Kind, r.Code = KindOK, resp.StatusCode
	return r
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func classify(err error) Kind {

	if msg := err.Error(); strings.Contains(msg, "protocol version not supported") ||
		strings.Contains(msg, "unsupported protocol version") {
		return KindUnsup
	}
	var cv *tls.CertificateVerificationError
	var ua x509.UnknownAuthorityError
	var hn x509.HostnameError
	if errors.As(err, &cv) || errors.As(err, &ua) || errors.As(err, &hn) {
		return KindSSL
	}
	return KindFail
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout() || errors.Is(err, context.DeadlineExceeded)
}
