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
	"errors"
	"fmt"
	"net/url"
	"os"
)

func ServerTLSConfig(certPath, keyPath, caPath, allowedURI string) (*tls.Config, error) {
	expected, err := url.Parse(allowedURI)
	if err != nil || expected == nil || expected.Scheme != "spiffe" ||
		expected.Host != "vpc-proxy.local" || expected.User != nil ||
		expected.RawQuery != "" || expected.Fragment != "" ||
		(expected.Path != "/egress/proxy-1" && expected.Path != "/egress/proxy-2") {
		return nil, errors.New("invalid allowed client SPIFFE URI")
	}
	leaf, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load serving keypair: %w", err)
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("load client CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("client CA contains no certificates")
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{leaf},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
		NextProtos:   []string{"http/1.1"},
		VerifyConnection: func(cs tls.ConnectionState) error {
			// Стандартный TLS уже проверил CA, EKU и срок действия.
			if len(cs.VerifiedChains) == 0 || len(cs.PeerCertificates) == 0 {
				return errors.New("verified client certificate required")
			}
			uris := cs.PeerCertificates[0].URIs
			if len(uris) != 1 || uris[0].String() != allowedURI {
				return errors.New("client identity rejected")
			}
			return nil
		},
	}, nil
}
