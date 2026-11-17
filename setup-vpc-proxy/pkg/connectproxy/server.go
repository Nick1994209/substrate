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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"go.ate.dev/substrate/setup-vpc-proxy/pkg/netpolicy"
)

type Config struct {
	Listen, TLSCert, TLSKey, ClientCA, AllowedClientURI, ProxyID, HealthListen string
	MaxTunnels                                                                 int
	DialTimeout, TunnelIdleTimeout, MaxTunnelDuration                          time.Duration
}
type Server struct {
	cfg     Config
	tls     *tls.Config
	slots   chan struct{}
	mu      sync.Mutex
	closing bool
	stopped chan struct{}
	conns   map[net.Conn]net.Conn
	dial    func(context.Context, string, string) (net.Conn, error)
	policy  func(netip.Addr, uint16) error
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.MaxTunnels < 1 || cfg.DialTimeout <= 0 || cfg.TunnelIdleTimeout <= 0 || cfg.MaxTunnelDuration <= 0 || cfg.ProxyID == "" {
		return nil, errors.New("invalid proxy limits or identity")
	}
	host, _, err := net.SplitHostPort(cfg.HealthListen)
	if err != nil {
		return nil, err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return nil, errors.New("health must bind loopback")
	}
	t, err := ServerTLSConfig(cfg.TLSCert, cfg.TLSKey, cfg.ClientCA, cfg.AllowedClientURI)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: cfg.DialTimeout}
	return &Server{cfg: cfg, tls: t, slots: make(chan struct{}, cfg.MaxTunnels), conns: make(map[net.Conn]net.Conn), stopped: make(chan struct{}), dial: d.DialContext, policy: netpolicy.CheckPublicTarget}, nil
}
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return err
	}
	health, err := net.Listen("tcp", s.cfg.HealthListen)
	if err != nil {
		ln.Close()
		return err
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	hs := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		closing := s.closing
		s.mu.Unlock()
		if closing || r.URL.Path != "/readyz" || r.Method != "GET" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ready\n"))
	}), ReadHeaderTimeout: 2 * time.Second}
	out := make(chan error, 2)
	go func() { out <- hs.Serve(health) }()
	go func() { out <- srv.Serve(tls.NewListener(limitListener(ln, 2*s.cfg.MaxTunnels), s.tls)) }()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-out:
	}
	s.close()
	_ = hs.Close()
	_ = srv.Close()
	if errors.Is(err, context.Canceled) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (s *Server) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closing {
		close(s.stopped)
		s.closing = true
	}
	for c, o := range s.conns {
		_ = c.Close()
		_ = o.Close()
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	deny := func(code int, reason string) {
		slog.Warn("connect_denied", "proxy_id", s.cfg.ProxyID, "peer_uri", s.cfg.AllowedClientURI, "destination", r.Host, "reason", reason)
		http.Error(w, http.StatusText(code), code)
	}
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 || len(r.TLS.PeerCertificates[0].URIs) != 1 || r.TLS.PeerCertificates[0].URIs[0].String() != s.cfg.AllowedClientURI {
		deny(403, "identity")
		return
	}
	if r.Method != http.MethodConnect {
		deny(405, "method")
		return
	}
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 || r.URL.User != nil || r.URL.Path != "" || r.URL.RawQuery != "" || r.URL.Fragment != "" || r.URL.Scheme != "" || r.Host != r.URL.Host || r.RequestURI != r.Host {
		deny(400, "authority or framing")
		return
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		deny(400, "authority")
		return
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		deny(400, "literal address required")
		return
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || strconv.FormatUint(n, 10) != port {
		deny(400, "port")
		return
	}
	if err = s.policy(ip, uint16(n)); err != nil {
		deny(403, "destination")
		return
	}
	s.mu.Lock()
	closing := s.closing
	s.mu.Unlock()
	if closing {
		deny(503, "shutdown")
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		deny(503, "budget")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.DialTimeout)
	origin, err := s.dial(ctx, "tcp", net.JoinHostPort(ip.Unmap().String(), port))
	cancel()
	if err != nil {
		deny(502, "dial")
		return
	}
	defer origin.Close()
	hj, ok := w.(http.Hijacker)
	if !ok {
		deny(500, "hijack unavailable")
		return
	}
	client, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	s.conns[client] = origin
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.conns, client); s.mu.Unlock() }()
	_ = client.SetDeadline(time.Time{})
	if _, err = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err = rw.Flush(); err != nil {
		return
	}
	start := time.Now()
	slog.Info("connect_open", "proxy_id", s.cfg.ProxyID, "peer_uri", s.cfg.AllowedClientURI, "destination", r.Host)
	active := make(chan struct{}, 1)
	done := make(chan struct{})
	cause := make(chan string, 1)
	stop := func(reason string) { cause <- reason; _ = client.Close(); _ = origin.Close() }
	touch := func() {
		select {
		case active <- struct{}{}:
		default:
		}
	}
	go func() {
		idle := time.NewTimer(s.cfg.TunnelIdleTimeout)
		defer idle.Stop()
		max := time.NewTimer(s.cfg.MaxTunnelDuration)
		defer max.Stop()
		for {
			select {
			case <-done:
				return
			case <-s.stopped:
				stop("shutdown")
				return
			case <-max.C:
				stop("max_duration")
				return
			case <-idle.C:
				stop("idle_timeout")
				return
			case <-active:
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(s.cfg.TunnelIdleTimeout)
			}
		}
	}()
	defer close(done)
	type copied struct {
		n   int64
		err error
		up  bool
	}
	results := make(chan copied, 2)
	go func() {
		n, e := io.Copy(activityWriter{origin, touch}, rw.Reader)
		if tcp, ok := origin.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		results <- copied{n, e, true}
	}()
	go func() {
		n, e := io.Copy(activityWriter{client, touch}, origin)
		if e == nil {
			if cw, ok := client.(interface{ CloseWrite() error }); ok {
				_ = cw.CloseWrite()
			}
		}
		results <- copied{n, e, false}
	}()
	first := <-results
	// Чистый EOF закрывает только направление; второе ограничено idle/max timers.
	if first.err != nil {
		_ = client.Close()
		_ = origin.Close()
	}
	second := <-results
	in, out := first.n, second.n
	if !first.up {
		in, out = second.n, first.n
	}
	reason := "complete"
	if first.err != nil || second.err != nil {
		reason = "stream_error"
	}
	select {
	case reason = <-cause:
	default:
	}
	slog.Info("connect_close", "reason", reason, "proxy_id", s.cfg.ProxyID, "peer_uri", s.cfg.AllowedClientURI, "destination", r.Host, "bytes_in", in, "bytes_out", out, "duration", time.Since(start).String())
}

type activityWriter struct {
	io.Writer
	touch func()
}

func (w activityWriter) Write(p []byte) (int, error) {
	n, e := w.Writer.Write(p)
	if n > 0 {
		w.touch()
	}
	return n, e
}
func CheckReady(address string) error {
	c := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	defer c.CloseIdleConnections()
	resp, err := c.Get("http://" + address + "/readyz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || string(b) != "ready\n" {
		return fmt.Errorf("not ready: %d", resp.StatusCode)
	}
	return nil
}

// Лимит включает TLS handshake и idle HTTP до первого CONNECT.
type cappedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}
type cappedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *cappedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
func limitListener(ln net.Listener, n int) net.Listener {
	return &cappedListener{Listener: ln, slots: make(chan struct{}, n), done: make(chan struct{})}
}
func (l *cappedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &cappedConn{Conn: c, release: func() { <-l.slots }}, nil
}
func (l *cappedListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}
