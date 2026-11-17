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
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"go.ate.dev/substrate/setup-vpc-proxy/pkg/bindings"
	r "go.ate.dev/substrate/setup-vpc-proxy/pkg/render"
)

func read(p string) []byte {
	b, e := os.ReadFile(p)
	if e != nil {
		log.Fatal(e)
	}
	return b
}
func main() {
	work := flag.String("work", "setup-vpc-proxy/.work", "")
	flag.Parse()
	if e := run(*work); e != nil {
		log.Fatal(e)
	}
}
func run(w string) error {
	root := "setup-vpc-proxy"
	out := filepath.Join(w, "rendered")
	if err := os.MkdirAll(out, 0700); err != nil {
		return err
	}
	routes := read(filepath.Join(w, "routes.yaml"))
	if _, e := bindings.LoadFile(filepath.Join(w, "routes.yaml")); e != nil {
		return e
	}
	var images struct {
		Gateway string `json:"gateway"`
		Proxy   string `json:"proxy"`
	}
	if e := json.Unmarshal(read(filepath.Join(w, "images.json")), &images); e != nil {
		return e
	}
	for _, v := range []string{images.Gateway, images.Proxy} {
		if e := r.CheckImage(v); e != nil {
			return e
		}
	}
	var cm, dep r.M
	if e := yaml.Unmarshal(read(filepath.Join(w, "baseline-configmap.yaml")), &cm); e != nil {
		return e
	}
	if e := yaml.Unmarshal(read(filepath.Join(w, "baseline-deployment.yaml")), &dep); e != nil {
		return e
	}
	if cm["kind"] != "ConfigMap" || dep["kind"] != "Deployment" || r.Map(cm["metadata"])["namespace"] != "ate-system" || r.Map(dep["metadata"])["namespace"] != "ate-system" || r.Map(cm["metadata"])["name"] != "atenet-egress" || r.Map(dep["metadata"])["name"] != "atenet-egress" {
		return fmt.Errorf("unexpected baseline object names")
	}
	data := r.Map(cm["data"])
	envoy, ok := data["envoy.yaml"].(string)
	if !ok {
		return fmt.Errorf("missing envoy.yaml")
	}
	patched, e := r.PatchEnvoy([]byte(envoy), read(root+"/config/envoy-patch.yaml"))
	if e != nil {
		return e
	}
	data["envoy.yaml"] = string(patched)
	hashInput := append(append(patched, routes...), []byte(images.Gateway+images.Proxy)...)
	for _, id := range []string{"1", "2"} {
		for _, file := range []string{"ca.crt", "client.crt", "server.crt"} {
			hashInput = append(hashInput, read(filepath.Join(w, "pki", id, file))...)
		}
	}
	hash := r.Hash(hashInput)
	if e = r.PatchDeployment(dep, images.Gateway, hash); e != nil {
		return e
	}
	r.StripRuntime(cm)
	r.StripRuntime(dep)
	objects := []r.M{cm, dep, {"apiVersion": "v1", "kind": "ConfigMap", "metadata": r.M{"name": "vpc-proxy-routes", "namespace": "ate-system"}, "data": r.M{"routes.yaml": string(routes)}}}
	var gateway bytes.Buffer
	enc := yaml.NewEncoder(&gateway)
	for _, v := range objects {
		if e = enc.Encode(v); e != nil {
			return e
		}
	}
	if e = os.WriteFile(filepath.Join(out, "gateway.yaml"), gateway.Bytes(), 0600); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(out, "envoy.yaml"), patched, 0600); e != nil {
		return e
	}
	t, e := template.ParseFiles(root + "/manifests/proxy.yaml.tmpl")
	if e != nil {
		return e
	}
	var proxies bytes.Buffer
	proxies.WriteString("apiVersion: v1\nkind: Namespace\nmetadata: {name: vpc-proxy, labels: {app.kubernetes.io/part-of: setup-vpc-proxy}}\n---\n")
	for _, id := range []string{"1", "2"} {
		if e = t.Execute(&proxies, struct{ ID, ProxyImage, CertHash string }{id, images.Proxy, r.Hash(read(filepath.Join(w, "pki", id, "server.crt")))}); e != nil {
			return e
		}
		proxies.WriteString("\n---\n")
	}
	if e = os.WriteFile(filepath.Join(out, "proxies.yaml"), proxies.Bytes(), 0600); e != nil {
		return e
	}
	var secrets bytes.Buffer
	se := yaml.NewEncoder(&secrets)
	for _, id := range []string{"1", "2"} {
		for _, role := range []string{"client", "server"} {
			name, ns := "vpc-proxy-client-"+id, "ate-system"
			if role == "server" {
				name, ns = "proxy-"+id+"-server-tls", "vpc-proxy"
			}
			dir := filepath.Join(w, "pki", id)
			v := r.M{"apiVersion": "v1", "kind": "Secret", "metadata": r.M{"name": name, "namespace": ns}, "type": "kubernetes.io/tls", "stringData": r.M{"tls.crt": string(read(filepath.Join(dir, role+".crt"))), "tls.key": string(read(filepath.Join(dir, role+".key"))), "ca.crt": string(read(filepath.Join(dir, "ca.crt")))}}
			if e = se.Encode(v); e != nil {
				return e
			}
		}
	}
	if e = os.WriteFile(filepath.Join(out, "secrets.yaml"), secrets.Bytes(), 0600); e != nil {
		return e
	}
	// Сетевые CIDR собирает render.sh из фактического кластера.
	network, e := r.PatchNetworkPolicies(read(filepath.Join(w, "networkpolicies.yaml")))
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(out, "networkpolicies.yaml"), network, 0600); e != nil {
		return e
	}
	if e = validation(out, cm, dep, secrets.Bytes()); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(out, "config.sha256"), []byte(hash+"\n"), 0600)
}

func validation(out string, cm, dep r.M, secrets []byte) error {
	clone := func(v r.M) r.M { b, _ := yaml.Marshal(v); var o r.M; _ = yaml.Unmarshal(b, &o); return o }
	c := clone(cm)
	r.Map(c["metadata"])["name"] = "vpc-proxy-validation"
	d := clone(dep)
	spec := r.Map(r.Map(r.Map(d["spec"])["template"])["spec"])
	var envoy r.M
	for _, v := range spec["containers"].([]any) {
		m := r.Map(v)
		if m["name"] == "envoy" {
			envoy = m
		}
	}
	if envoy == nil {
		return fmt.Errorf("missing envoy container")
	}
	envoy["args"] = []string{"--mode", "validate", "-c", "/etc/envoy/envoy.yaml"}
	for _, k := range []string{"readinessProbe", "livenessProbe", "startupProbe", "lifecycle", "ports"} {
		delete(envoy, k)
	}
	spec["containers"] = []any{envoy}
	delete(spec, "initContainers")
	spec["restartPolicy"] = "Never"
	spec["automountServiceAccountToken"] = false
	spec["activeDeadlineSeconds"] = 120
	for _, v := range spec["volumes"].([]any) {
		m := r.Map(v)
		if cfg := r.Map(m["configMap"]); cfg != nil && cfg["name"] == "atenet-egress" {
			cfg["name"] = "vpc-proxy-validation"
		}
		if sec := r.Map(m["secret"]); sec != nil {
			for _, id := range []string{"1", "2"} {
				if sec["secretName"] == "vpc-proxy-client-"+id {
					sec["secretName"] = "vpc-proxy-validation-client-" + id
				}
			}
		}
	}
	pod := r.M{"apiVersion": "v1", "kind": "Pod", "metadata": r.M{"name": "vpc-proxy-validation", "namespace": "ate-system", "labels": r.M{"app": "vpc-proxy-validation"}}, "spec": spec}
	var b bytes.Buffer
	en := yaml.NewEncoder(&b)
	if e := en.Encode(c); e != nil {
		return e
	}
	service := r.M{"apiVersion": "v1", "kind": "Service", "metadata": r.M{"name": "vpc-proxy-validation", "namespace": "ate-system"}, "spec": r.M{"clusterIP": "None", "selector": r.M{"app": "vpc-proxy-validation"}, "ports": []any{r.M{"port": 443, "targetPort": 443}}}}
	if e := en.Encode(service); e != nil {
		return e
	}
	if e := en.Encode(pod); e != nil {
		return e
	}
	dec := yaml.NewDecoder(bytes.NewReader(secrets))
	for {
		var v r.M
		e := dec.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		md := r.Map(v["metadata"])
		if md["namespace"] != "ate-system" {
			continue
		}
		md["name"] = strings.Replace(md["name"].(string), "vpc-proxy-client-", "vpc-proxy-validation-client-", 1)
		if e = en.Encode(v); e != nil {
			return e
		}
	}
	return os.WriteFile(filepath.Join(out, "validation.yaml"), b.Bytes(), 0600)
}
