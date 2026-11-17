// Copyright 2026 The Agent Substrate Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connectproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func request(t *testing.T, address string, cfg *tls.Config, authority, extra, payload string) (*tls.Conn, *bufio.Reader, int) {
	t.Helper()
	c, e := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", address, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Close() })
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	_, e = io.WriteString(c, "CONNECT "+authority+" HTTP/1.1\r\nHost: "+authority+"\r\n"+extra+"\r\n"+payload)
	if e != nil {
		t.Fatal(e)
	}
	br := bufio.NewReader(c)
	resp, e := http.ReadResponse(br, &http.Request{Method: "CONNECT"})
	if e != nil {
		t.Fatal(e)
	}
	return c, br, resp.StatusCode
}
func TestTunnel(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	origin, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer origin.Close()
	s.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "1.1.1.1:443" {
			t.Errorf("unexpected target %s", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, origin.Addr().String())
	}
	got := make(chan string, 1)
	go func() {
		c, e := origin.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		b := make([]byte, 5)
		_, e = io.ReadFull(c, b)
		if e != nil {
			return
		}
		got <- string(b)
		_, _ = c.Write([]byte("world"))
		_, _ = io.Copy(io.Discard, c)
	}()
	address := listen(t, s)
	client, reader, code := request(t, address, cfg, "1.1.1.1:443", "", "hello")
	if code != 200 {
		t.Fatal(code)
	}
	b := make([]byte, 5)
	if _, e = io.ReadFull(reader, b); e != nil || string(b) != "world" {
		t.Fatal(string(b), e)
	}
	select {
	case v := <-got:
		if v != "hello" {
			t.Fatal(v)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("buffered payload lost")
	}
	_, _, code = request(t, address, cfg, "1.1.1.1:443", "", "")
	if code != 503 {
		t.Fatal("budget", code)
	}
	s.close()
	if _, e = client.Read(make([]byte, 1)); e == nil {
		t.Fatal("shutdown did not close tunnel")
	}
}
func TestRejectRequests(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	s.dial = func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("unavailable") }
	address := listen(t, s)
	for _, tc := range []struct {
		authority, headers string
		code               int
	}{
		{"example.com:443", "", 400}, {"127.0.0.1:443", "", 403}, {"[::1]:443", "", 403}, {"1.1.1.1:22", "", 403}, {"1.1.1.1:443", "Content-Length: 1\r\n", 400}, {"1.1.1.1:443", "Transfer-Encoding: chunked\r\n", 400}, {"1.1.1.1:443", "", 502},
	} {
		t.Run(tc.authority+tc.headers, func(t *testing.T) {
			_, _, code := request(t, address, cfg, tc.authority, tc.headers, "")
			if code != tc.code {
				t.Fatalf("%d want %d", code, tc.code)
			}
		})
	}
}
func TestTunnelTimeout(t *testing.T) {
	for _, name := range []string{"idle", "max"} {
		t.Run(name, func(t *testing.T) {
			s, ca, cfg := fixture(t)
			cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
			s.cfg.TunnelIdleTimeout = 20 * time.Millisecond
			s.cfg.MaxTunnelDuration = time.Second
			if name == "max" {
				s.cfg.TunnelIdleTimeout = time.Second
				s.cfg.MaxTunnelDuration = 20 * time.Millisecond
			}
			server, peer := net.Pipe()
			defer peer.Close()
			s.dial = func(context.Context, string, string) (net.Conn, error) { return server, nil }
			s.policy = func(netip.Addr, uint16) error { return nil }
			address := listen(t, s)
			c, br, code := request(t, address, cfg, "1.1.1.1:443", "", "")
			if code != 200 {
				t.Fatal(code)
			}
			_ = c.SetDeadline(time.Now().Add(time.Second))
			_, e := br.ReadByte()
			if e == nil || strings.Contains(e.Error(), "i/o timeout") {
				t.Fatalf("tunnel did not close: %v", e)
			}
		})
	}
}

func TestRunCancellation(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	free := func() string {
		t.Helper()
		ln, e := net.Listen("tcp4", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		return addr
	}
	s.cfg.Listen = free()
	s.cfg.HealthListen = free()
	server, peer := net.Pipe()
	defer peer.Close()
	s.dial = func(context.Context, string, string) (net.Conn, error) { return server, nil }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for CheckReady(s.cfg.HealthListen) != nil {
		select {
		case e := <-done:
			t.Fatal(e)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("health startup timeout")
		}
	}
	c, _, code := request(t, s.cfg.Listen, cfg, "1.1.1.1:443", "", "")
	if code != 200 {
		t.Fatal(code)
	}
	cancel()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
	if _, e := c.Read(make([]byte, 1)); e == nil {
		t.Fatal("active tunnel survived shutdown")
	}
}
func TestListenerBudget(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(tls.NewListener(limitListener(ln, 1), s.tls)) }()
	defer srv.Close()
	first, e := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", ln.Addr().String(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer first.Close()
	second, e := tls.DialWithDialer(&net.Dialer{Timeout: 50 * time.Millisecond}, "tcp", ln.Addr().String(), cfg)
	if e == nil {
		second.Close()
		t.Fatal("idle TLS connection bypassed listener budget")
	}
	if ne, ok := e.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("expected budget timeout, got %v", e)
	}
	_ = first.Close()
	next, e := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", ln.Addr().String(), cfg)
	if e != nil {
		t.Fatal("slot not released", e)
	}
	_ = next.Close()
}

func TestHalfClosePreservesResponse(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	s.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, ln.Addr().String())
	}
	payload := strings.Repeat("early-payload", 8192)
	originResult := make(chan error, 1)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			originResult <- e
			return
		}
		defer c.Close()
		b, e := io.ReadAll(c)
		if e == nil && string(b) != payload {
			e = errors.New("truncated request")
		}
		if e == nil {
			_, e = c.Write(b)
		}
		originResult <- e
	}()
	address := listen(t, s)
	client, br, code := request(t, address, cfg, "1.1.1.1:443", "", payload)
	if code != 200 {
		t.Fatal(code)
	}
	if e = client.CloseWrite(); e != nil {
		t.Fatal(e)
	}
	b, e := io.ReadAll(br)
	if e != nil || string(b) != payload {
		t.Fatalf("half-close response bytes=%d: %v", len(b), e)
	}
	if e = <-originResult; e != nil {
		t.Fatal(e)
	}
}

func TestOriginHalfClosePreservesUpload(t *testing.T) {
	s, ca, cfg := fixture(t)
	cfg.Certificates = []tls.Certificate{issue(t, ca, clientURI, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	s.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, ln.Addr().String())
	}
	payload := strings.Repeat("upload", 8192)
	originResult := make(chan error, 1)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			originResult <- e
			return
		}
		defer c.Close()
		_, e = c.Write([]byte("hello"))
		if e == nil {
			e = c.(*net.TCPConn).CloseWrite()
		}
		if e == nil {
			var b []byte
			b, e = io.ReadAll(c)
			if e == nil && string(b) != payload {
				e = errors.New("truncated upload after origin half-close")
			}
		}
		originResult <- e
	}()
	address := listen(t, s)
	client, br, code := request(t, address, cfg, "1.1.1.1:443", "", "")
	if code != 200 {
		t.Fatal(code)
	}
	b, e := io.ReadAll(br)
	if e != nil || string(b) != "hello" {
		t.Fatal(string(b), e)
	}
	if _, e = io.WriteString(client, payload); e != nil {
		t.Fatal(e)
	}
	if e = client.CloseWrite(); e != nil {
		t.Fatal(e)
	}
	select {
	case e = <-originResult:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload stalled")
	}
}
