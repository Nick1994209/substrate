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
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.ate.dev/substrate/setup-vpc-proxy/pkg/devpki"
)

const clientURI = "spiffe://vpc-proxy.local/egress/proxy-1"

func fixture(t *testing.T) (*Server, *devpki.CA, *tls.Config) {
	t.Helper()
	ca, e := devpki.NewCA("test")
	if e != nil {
		t.Fatal(e)
	}
	cert, key, e := ca.Issue("spiffe://vpc-proxy.local/proxy/proxy-1", "localhost", x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	for n, b := range map[string][]byte{"tls.crt": cert, "tls.key": key, "ca.crt": ca.PEM()} {
		if e = os.WriteFile(filepath.Join(dir, n), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	s, e := NewServer(Config{Listen: "127.0.0.1:0", TLSCert: filepath.Join(dir, "tls.crt"), TLSKey: filepath.Join(dir, "tls.key"), ClientCA: filepath.Join(dir, "ca.crt"), AllowedClientURI: clientURI, ProxyID: "proxy-1", HealthListen: "127.0.0.1:0", MaxTunnels: 1, DialTimeout: time.Second, TunnelIdleTimeout: time.Second, MaxTunnelDuration: 2 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	return s, ca, &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "localhost", NextProtos: []string{"http/1.1"}}
}
func issue(t *testing.T, ca *devpki.CA, uri string, eku x509.ExtKeyUsage, until time.Time) tls.Certificate {
	t.Helper()
	c, k, e := ca.Issue(uri, "", eku, until)
	if e != nil {
		t.Fatal(e)
	}
	v, e := tls.X509KeyPair(c, k)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func listen(t *testing.T, s *Server) string {
	t.Helper()
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(tls.NewListener(ln, s.tls)) }()
	t.Cleanup(func() { s.close(); _ = srv.Close() })
	return ln.Addr().String()
}
func TestMTLS(t *testing.T) {
	s, ca, base := fixture(t)
	address := listen(t, s)
	other, e := devpki.NewCA("other")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, uri string
		ca        *devpki.CA
		eku       x509.ExtKeyUsage
		until     time.Time
		ok        bool
	}{
		{"valid", clientURI, ca, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour), true},
		{"wrong uri", "spiffe://vpc-proxy.local/egress/proxy-2", ca, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour), false},
		{"wrong ca", clientURI, other, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour), false},
		{"expired", clientURI, ca, x509.ExtKeyUsageClientAuth, time.Now().Add(-time.Minute), false},
		{"wrong eku", clientURI, ca, x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour), false},
		{"no cert", "", nil, 0, time.Time{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base.Clone()
			if tc.ca != nil {
				cfg.Certificates = []tls.Certificate{issue(t, tc.ca, tc.uri, tc.eku, tc.until)}
			}
			tr := &http.Transport{TLSClientConfig: cfg, Proxy: nil}
			defer tr.CloseIdleConnections()
			c := &http.Client{Transport: tr, Timeout: 2 * time.Second}
			resp, e := c.Get("https://" + address + "/")
			if e == nil {
				defer resp.Body.Close()
				_, _ = io.Copy(io.Discard, resp.Body)
			}
			if tc.ok && (e != nil || resp.StatusCode != 405 || resp.ProtoMajor != 1 || resp.TLS.NegotiatedProtocol != "http/1.1") {
				t.Fatalf("valid: %v %v", resp, e)
			}
			if !tc.ok && e == nil {
				t.Fatal("accepted invalid TLS identity")
			}
		})
	}
	t.Run("plaintext", func(t *testing.T) {
		c, e := net.DialTimeout("tcp", address, time.Second)
		if e != nil {
			t.Fatal(e)
		}
		defer c.Close()
		_ = c.SetDeadline(time.Now().Add(time.Second))
		_, _ = io.WriteString(c, "CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\n\r\n")
		b, _ := io.ReadAll(c)
		if string(b) == "HTTP/1.1 200 Connection Established\r\n\r\n" {
			t.Fatal("plaintext tunnel")
		}
	})
}
