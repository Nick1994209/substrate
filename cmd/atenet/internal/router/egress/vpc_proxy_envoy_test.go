//go:build vpcproxy_envoy

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

package egress

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ateapi "go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/atenet/internal/router/extproc"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/bindings"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/connectproxy"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/devpki"
	render "go.ate.dev/substrate/setup-vpc-proxy/pkg/render"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type vpcFixtureAPI struct{ ateapi.ControlClient }

func (vpcFixtureAPI) GetActor(_ context.Context, req *ateapi.GetActorRequest, _ ...grpc.CallOption) (*ateapi.Actor, error) {
	a := runningActor()
	a.Metadata.Name = req.Actor.Name
	a.Metadata.Atespace = req.Actor.Atespace
	if req.Actor.Name == "actor-2" {
		a.Metadata.Uid = "22222222-2222-4222-8222-222222222222"
	}
	return a, nil
}

type inspectedVPCHandler struct {
	*Handler
	unsanitized atomic.Int32
	calls       atomic.Int32
}

func (h *inspectedVPCHandler) HandleRequestHeaders(ctx context.Context, md *extproc.RequestMetadata) (extproc.Result, error) {
	h.calls.Add(1)
	if md.Header("x-ate-vpc-cluster") != "" {
		h.unsanitized.Add(1)
	}
	return h.Handler.HandleRequestHeaders(ctx, md)
}
func TestVPCProxyEnvoy(t *testing.T) {
	image := os.Getenv("VPC_PROXY_ENVOY_IMAGE")
	if !strings.Contains(image, "@sha256:") {
		t.Fatal("VPC_PROXY_ENVOY_IMAGE must be the pinned baseline Envoy image")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	docker := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "docker", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		b, e := cmd.Output()
		if e != nil {
			t.Fatalf("docker %v: %v: %s %s", args, e, b, stderr.String())
		}
		return strings.TrimSpace(string(b))
	}
	docker("info")
	root := "../../../../../"
	project := filepath.Join(root, "setup-vpc-proxy")
	b, e := os.ReadFile(filepath.Join(root, "manifests/ate-install/atenet-egress.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	var envoy string
	for {
		var v render.M
		e = decoder.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if v["kind"] == "ConfigMap" && render.Map(v["metadata"])["name"] == "atenet-egress" {
			envoy = render.Map(v["data"])["envoy.yaml"].(string)
		}
	}
	patch, e := os.ReadFile(filepath.Join(project, "config/envoy-patch.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	patched, e := render.PatchEnvoy([]byte(envoy), patch)
	if e != nil {
		t.Fatal(e)
	}
	var cfg render.M
	_ = yaml.Unmarshal(patched, &cfg)
	dir := t.TempDir()
	write := func(n string, b []byte) string {
		t.Helper()
		p := filepath.Join(dir, n)
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	actorCA, e := devpki.NewCA("actor")
	if e != nil {
		t.Fatal(e)
	}
	write("actor-ca.crt", actorCA.PEM())
	serving, e := devpki.NewCA("gateway")
	if e != nil {
		t.Fatal(e)
	}
	c, k, e := serving.Issue("spiffe://test/gateway", "localhost", x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	write("gateway.crt", c)
	write("gateway.key", k)
	roots := x509.NewCertPool()
	roots.AddCert(actorCA.Cert)
	routes := []bindings.Route{{Actor: bindings.ActorKey{Atespace: "default", Name: "actor-1", UID: testEgressActorUID}, Mode: bindings.ModeProxy, Cluster: "vpc_proxy_1"}, {Actor: bindings.ActorKey{Atespace: "default", Name: "actor-2", UID: "22222222-2222-4222-8222-222222222222"}, Mode: bindings.ModeProxy, Cluster: "vpc_proxy_2"}, {Actor: bindings.ActorKey{Atespace: "default", Name: "actor-3", UID: "33333333-3333-4333-8333-333333333333"}, Mode: bindings.ModeDirect}}
	reg, e := bindings.New(bindings.FileConfig{APIVersion: "vpc-proxy.substrate.local/v1alpha1", Kind: "ActorEgressRoutes", Routes: routes})
	if e != nil {
		t.Fatal(e)
	}
	inspected := &inspectedVPCHandler{Handler: New(vpcFixtureAPI{}, func() (*x509.CertPool, error) { return roots, nil }, 0, nil, WithVPCProxyRegistry(reg))}
	gs := extproc.NewServer(0, nil, extproc.Handlers{extproc.DirectionEgress: inspected}).NewGRPCServer()
	defer gs.Stop()
	ln, e := net.Listen("tcp4", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	go func() { _ = gs.Serve(ln) }()
	static := render.Map(cfg["static_resources"])
	listeners := static["listeners"].([]any)
	outer := render.Map(listeners[0])
	if outer["name"] != "egress" {
		t.Fatal("unexpected outer listener")
	}
	chain := render.Map(outer["filter_chains"].([]any)[0])
	hcm := render.Map(render.Map(chain["filters"].([]any)[0])["typed_config"])
	// Fixture изолирует CONNECT routing от MITM store. Полный config проверяется отдельно live validation.
	var filters []any
	for _, v := range hcm["http_filters"].([]any) {
		m := render.Map(v)
		if m["name"] == "envoy.filters.http.dynamic_modules" || m["name"] == "envoy.filters.http.set_filter_state" {
			continue
		}
		filters = append(filters, v)
	}
	hcm["http_filters"] = filters
	delete(hcm, "access_log")
	delete(hcm, "access_log_options")
	outer["address"] = render.M{"socket_address": render.M{"address": "0.0.0.0", "port_value": 8443}}
	chain["transport_socket"] = render.M{"name": "envoy.transport_sockets.tls", "typed_config": render.M{"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext", "require_client_certificate": true, "common_tls_context": render.M{"tls_certificates": []any{render.M{"certificate_chain": render.M{"filename": "/fixture/gateway.crt"}, "private_key": render.M{"filename": "/fixture/gateway.key"}}}, "validation_context": render.M{"trusted_ca": render.M{"filename": "/fixture/actor-ca.crt"}}}}}
	socket := func(port int) render.M {
		return render.M{"address": render.M{"socket_address": render.M{"address": "host.docker.internal", "port_value": port}}}
	}
	cluster := func(name string, port int) render.M {
		return render.M{"name": name, "type": "STRICT_DNS", "connect_timeout": "1s", "load_assignment": render.M{"cluster_name": name, "endpoints": []any{render.M{"lb_endpoints": []any{render.M{"endpoint": socket(port)}}}}}}
	}
	ep := cluster("ext_proc_server", ln.Addr().(*net.TCPAddr).Port)
	ep["http2_protocol_options"] = render.M{}
	var clusters []any
	clusters = append(clusters, ep, cluster("mitm_internal", 1))
	for _, id := range []string{"1", "2"} {
		ca, e := devpki.NewCA("binding-" + id)
		if e != nil {
			t.Fatal(e)
		}
		write("ca-"+id+".crt", ca.PEM())
		serverURI := "spiffe://vpc-proxy.local/proxy/proxy-" + id
		// Binding 2 предъявляет trusted CA leaf с неверным server URI.
		if id == "2" {
			serverURI = "spiffe://vpc-proxy.local/proxy/wrong"
		}
		cert, key, e := ca.Issue(serverURI, "localhost", x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		cp, kp := write("server-"+id+".crt", cert), write("server-"+id+".key", key)
		cert, key, e = ca.Issue("spiffe://vpc-proxy.local/egress/proxy-"+id, "", x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		write("client-"+id+".crt", cert)
		write("client-"+id+".key", key)
		tlsCfg, e := connectproxy.ServerTLSConfig(cp, kp, filepath.Join(dir, "ca-"+id+".crt"), "spiffe://vpc-proxy.local/egress/proxy-"+id)
		if e != nil {
			t.Fatal(e)
		}
		bl, e := net.Listen("tcp4", "0.0.0.0:0")
		if e != nil {
			t.Fatal(e)
		}
		hs := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "CONNECT" {
				http.Error(w, "method", 405)
				return
			}
			c, rw, e := w.(http.Hijacker).Hijack()
			if e != nil {
				return
			}
			defer c.Close()
			_, _ = rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\nfixture")
			_ = rw.Flush()
		}), ReadHeaderTimeout: time.Second}
		defer hs.Close()
		go func() { _ = hs.Serve(tls.NewListener(bl, tlsCfg)) }()
		pc := cluster("vpc_proxy_"+id, bl.Addr().(*net.TCPAddr).Port)
		pc["transport_socket"] = render.M{"name": "envoy.transport_sockets.tls", "typed_config": render.M{"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext", "sni": "localhost", "common_tls_context": render.M{"alpn_protocols": []string{"http/1.1"}, "tls_params": render.M{"tls_minimum_protocol_version": "TLSv1_3", "tls_maximum_protocol_version": "TLSv1_3"}, "tls_certificates": []any{render.M{"certificate_chain": render.M{"filename": "/fixture/client-" + id + ".crt"}, "private_key": render.M{"filename": "/fixture/client-" + id + ".key"}}}, "validation_context": render.M{"trusted_ca": render.M{"filename": "/fixture/ca-" + id + ".crt"}, "match_typed_subject_alt_names": []any{render.M{"san_type": "URI", "matcher": render.M{"exact": "spiffe://vpc-proxy.local/proxy/proxy-" + id}}}}}}}
		clusters = append(clusters, pc)
	}
	cfg = render.M{"static_resources": render.M{"listeners": []any{outer}, "clusters": clusters}}
	b, e = yaml.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	write("envoy.yaml", b)
	// Docker читает только throwaway fixture keys. Runtime CA здесь не используется.
	args := []string{"run", "-d", "--label", "setup-vpc-proxy.test=envoy", "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())}
	if runtime.GOOS == "linux" {
		args = append(args, "--add-host=host.docker.internal:host-gateway")
	}
	args = append(args, "-p", "127.0.0.1::8443", "-v", dir+":/fixture:ro", image, "--mode", "serve", "--log-level", "warning", "--component-log-level", "connection:debug", "-c", "/fixture/envoy.yaml")
	container := docker(args...)
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := exec.Command("docker", "logs", container).CombinedOutput()
			t.Logf("Envoy fixture logs: %s", b)
		}
		_, _ = exec.Command("docker", "rm", "-f", container).CombinedOutput()
	})
	port := docker("port", container, "8443/tcp")
	address := strings.TrimSpace(strings.Split(port, "\n")[0])
	trust := x509.NewCertPool()
	trust.AddCert(serving.Cert)
	for _, id := range []string{"1", "2"} {
		cert, key, e := actorCA.Issue("spiffe://substrate-actor.local/worker-for-actor/default/actor-"+id, "", x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour))
		if e != nil {
			t.Fatal(e)
		}
		pair, e := tls.X509KeyPair(cert, key)
		if e != nil {
			t.Fatal(e)
		}
		tc := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: trust, ServerName: "localhost", Certificates: []tls.Certificate{pair}, NextProtos: []string{"http/1.1"}}
		var c *tls.Conn
		for {
			c, e = tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", address, tc)
			if e == nil {
				break
			}
			if ctx.Err() != nil {
				t.Fatal(e, docker("logs", container))
			}
		}
		_ = c.SetDeadline(time.Now().Add(8 * time.Second))
		_, e = fmt.Fprint(c, "CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\nx-ate-vpc-cluster: vpc_proxy_2\r\n\r\n")
		if e != nil {
			t.Fatal(e)
		}
		resp, e := http.ReadResponse(bufio.NewReader(c), &http.Request{Method: "CONNECT"})
		defer c.Close()
		if e != nil {
			t.Fatal(e)
		}
		if id == "1" && resp.StatusCode != 200 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			t.Fatalf("sanitized authenticated route failed: %d: %s", resp.StatusCode, b)
		}
		if id == "2" && resp.StatusCode < 400 {
			t.Fatalf("wrong trusted server URI accepted: %d", resp.StatusCode)
		}
	}
	if inspected.calls.Load() != 2 || inspected.unsanitized.Load() != 0 {
		t.Fatalf("outer sanitation calls=%d unsanitized=%d", inspected.calls.Load(), inspected.unsanitized.Load())
	}
	evidence := os.Getenv("VPC_PROXY_EVIDENCE")
	if evidence != "" {
		for _, name := range []string{"envoy-san", "outer-sanitation"} {
			b, _ := json.Marshal(map[string]string{name: "PASS", "level": "LOCAL_INTEGRATION"})
			if e = os.WriteFile(filepath.Join(evidence, name+".json"), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
