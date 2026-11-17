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

package devpki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type CA struct {
	Cert *x509.Certificate
	Key  *ecdsa.PrivateKey
}

func NewCA(name string) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	c := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(7 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, MaxPathLenZero: true}
	der, err := x509.CreateCertificate(rand.Reader, c, c, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	return &CA{cert, key}, err
}
func (c *CA) Issue(uri, dns string, eku x509.ExtKeyUsage, until time.Time) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, nil, err
	}
	leaf := &x509.Certificate{SerialNumber: serial, NotBefore: until.Add(-24 * time.Hour), NotAfter: until, URIs: []*url.URL{u}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{eku}, BasicConstraintsValid: true}
	if dns != "" {
		leaf.DNSNames = []string{dns}
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, c.Cert, &key.PublicKey, c.Key)
	if err != nil {
		return nil, nil, err
	}
	k, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: k}), nil
}
func (c *CA) PEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Cert.Raw})
}
func Generate(dir string) error {
	// Создание каталога эксклюзивно: повторный запуск не заменяет рабочие ключи.
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	for _, id := range []string{"1", "2"} {
		ca, err := NewCA("vpc-proxy-" + id)
		if err != nil {
			return err
		}
		base := filepath.Join(dir, id)
		if err = os.Mkdir(base, 0700); err != nil {
			return err
		}
		key, err := x509.MarshalPKCS8PrivateKey(ca.Key)
		if err != nil {
			return err
		}
		files := map[string][]byte{"ca.crt": ca.PEM(), "ca.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})}
		for _, role := range []string{"client", "server"} {
			uri := "spiffe://vpc-proxy.local/egress/proxy-" + id
			dns := ""
			eku := x509.ExtKeyUsageClientAuth
			if role == "server" {
				uri = "spiffe://vpc-proxy.local/proxy/proxy-" + id
				dns = "proxy-vpc-pod-" + id + ".vpc-proxy.svc.cluster.local"
				eku = x509.ExtKeyUsageServerAuth
			}
			cert, key, err := ca.Issue(uri, dns, eku, time.Now().Add(24*time.Hour))
			if err != nil {
				return err
			}
			files[role+".crt"] = cert
			files[role+".key"] = key
		}
		for name, b := range files {
			if err = os.WriteFile(filepath.Join(base, name), b, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func NegativeClients(pkiDir, out string) error {
	b, err := os.ReadFile(filepath.Join(pkiDir, "1", "ca.crt"))
	if err != nil {
		return err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return fmt.Errorf("invalid CA certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	b, err = os.ReadFile(filepath.Join(pkiDir, "1", "ca.key"))
	if err != nil {
		return err
	}
	block, _ = pem.Decode(b)
	if block == nil {
		return fmt.Errorf("invalid CA key")
	}
	raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	key, ok := raw.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("expected ECDSA CA key")
	}
	ca := &CA{cert, key}
	foreign, err := NewCA("negative-test")
	if err != nil {
		return err
	}
	if err = os.Mkdir(out, 0700); err != nil {
		return err
	}
	for _, tc := range []struct {
		name, uri string
		ca        *CA
		eku       x509.ExtKeyUsage
		until     time.Time
	}{
		{"wrong-uri", "spiffe://vpc-proxy.local/egress/proxy-2", ca, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour)},
		{"expired", "spiffe://vpc-proxy.local/egress/proxy-1", ca, x509.ExtKeyUsageClientAuth, time.Now().Add(-time.Minute)},
		{"wrong-eku", "spiffe://vpc-proxy.local/egress/proxy-1", ca, x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour)},
		{"foreign-ca", "spiffe://vpc-proxy.local/egress/proxy-1", foreign, x509.ExtKeyUsageClientAuth, time.Now().Add(time.Hour)},
	} {
		cert, key, err := tc.ca.Issue(tc.uri, "", tc.eku, tc.until)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(out, tc.name+".crt"), cert, 0600); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(out, tc.name+".key"), key, 0600); err != nil {
			return err
		}
	}
	return nil
}

func Check(dir string) error {
	for _, id := range []string{"1", "2"} {
		b, err := os.ReadFile(filepath.Join(dir, id, "ca.crt"))
		if err != nil {
			return err
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(b) {
			return fmt.Errorf("invalid binding CA")
		}
		for _, role := range []string{"client", "server"} {
			pair, err := tls.LoadX509KeyPair(filepath.Join(dir, id, role+".crt"), filepath.Join(dir, id, role+".key"))
			if err != nil {
				return err
			}
			leaf, err := x509.ParseCertificate(pair.Certificate[0])
			if err != nil {
				return err
			}
			uri := "spiffe://vpc-proxy.local/egress/proxy-" + id
			opts := x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
			if role == "server" {
				uri = "spiffe://vpc-proxy.local/proxy/proxy-" + id
				opts.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
				opts.DNSName = "proxy-vpc-pod-" + id + ".vpc-proxy.svc.cluster.local"
			}
			if leaf.IsCA || len(leaf.URIs) != 1 || leaf.URIs[0].String() != uri || time.Until(leaf.NotAfter) < time.Hour {
				return fmt.Errorf("binding %s %s certificate identity or lifetime invalid", id, role)
			}
			if _, err = leaf.Verify(opts); err != nil {
				return err
			}
		}
	}
	return nil
}
