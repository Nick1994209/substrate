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
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	target := flag.String("address", "", "")
	ca := flag.String("ca", "", "")
	cert := flag.String("cert", "", "")
	key := flag.String("key", "", "")
	name := flag.String("server-name", "", "")
	uri := flag.String("server-uri", "", "")
	authority := flag.String("authority", "1.1.1.1:443", "")
	serve := flag.String("serve", "", "")
	plain := flag.Bool("http", false, "")
	waitHTTP := flag.Bool("wait-http", false, "Wait for positive HTTP control within 20 seconds")
	tcp := flag.Bool("tcp", false, "")
	flag.Parse()
	if *tcp {
		c, e := net.DialTimeout("tcp", *target, 2*time.Second)
		if e != nil {
			log.Fatal(e)
		}
		_ = c.Close()
		fmt.Println("tcp_connected")
		return
	}
	if *serve != "" {
		s := &http.Server{Addr: *serve, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("probe-ok")) }), ReadHeaderTimeout: time.Second, IdleTimeout: time.Second}
		log.Fatal(s.ListenAndServe())
	}
	if *plain {
		c := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second}
		deadline := time.Now().Add(20 * time.Second)
		var v *http.Response
		var e error
		for {
			v, e = c.Get("http://" + *target + "/")
			if e == nil {
				break
			}
			if !*waitHTTP || time.Now().After(deadline) {
				log.Fatal(e)
			}
		}

		defer v.Body.Close()
		b, e := io.ReadAll(io.LimitReader(v.Body, 32))
		if e != nil || v.StatusCode != 200 || string(b) != "probe-ok" {
			log.Fatal("probe response invalid")
		}
		return
	}
	roots := x509.NewCertPool()
	b, e := os.ReadFile(*ca)
	if e != nil {
		log.Fatal(e)
	}
	if !roots.AppendCertsFromPEM(b) {
		log.Fatal("invalid CA")
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: *name, NextProtos: []string{"http/1.1"}, VerifyConnection: func(cs tls.ConnectionState) error {
		if len(cs.VerifiedChains) == 0 || len(cs.PeerCertificates) == 0 || len(cs.PeerCertificates[0].URIs) != 1 || cs.PeerCertificates[0].URIs[0].String() != *uri {
			return errors.New("server URI rejected")
		}
		return nil
	}}
	if *cert != "" {
		v, e := tls.LoadX509KeyPair(*cert, *key)
		if e != nil {
			log.Fatal(e)
		}
		cfg.Certificates = []tls.Certificate{v}
	}
	c, e := tls.DialWithDialer(&net.Dialer{Timeout: 3 * time.Second}, "tcp", *target, cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	_, e = fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", *authority, *authority)
	if e != nil {
		log.Fatal(e)
	}
	resp, e := http.ReadResponse(bufio.NewReader(c), &http.Request{Method: "CONNECT"})
	if e != nil {
		log.Fatal(e)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("CONNECT %d", resp.StatusCode)
	}
	fmt.Println("CONNECT OK")
}
