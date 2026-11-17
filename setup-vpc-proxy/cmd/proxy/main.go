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

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.ate.dev/substrate/setup-vpc-proxy/pkg/connectproxy"
)

func main() {
	var c connectproxy.Config
	var check bool
	flag.StringVar(&c.Listen, "listen", ":8443", "")
	flag.StringVar(&c.TLSCert, "tls-cert", "/etc/proxy/tls/tls.crt", "")
	flag.StringVar(&c.TLSKey, "tls-key", "/etc/proxy/tls/tls.key", "")
	flag.StringVar(&c.ClientCA, "client-ca", "/etc/proxy/tls/ca.crt", "")
	flag.StringVar(&c.AllowedClientURI, "allowed-client-uri", "", "Exact client URI SAN")
	flag.StringVar(&c.ProxyID, "proxy-id", "", "Binding name")
	flag.IntVar(&c.MaxTunnels, "max-tunnels", 128, "")
	flag.DurationVar(&c.DialTimeout, "dial-timeout", 5*time.Second, "")
	flag.DurationVar(&c.TunnelIdleTimeout, "tunnel-idle-timeout", 60*time.Second, "")
	flag.DurationVar(&c.MaxTunnelDuration, "max-tunnel-duration", 5*time.Minute, "")
	flag.StringVar(&c.HealthListen, "health-listen", "127.0.0.1:9090", "")
	flag.BoolVar(&check, "check-ready", false, "")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if check {
		if err := connectproxy.CheckReady(c.HealthListen); err != nil {
			slog.Error("readiness", "error", err)
			os.Exit(1)
		}
		return
	}
	s, err := connectproxy.NewServer(c)
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err = s.Run(ctx); err != nil {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}
