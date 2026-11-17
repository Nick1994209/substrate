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

package render

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

// Gateway получает явные DNS/API/OTLP разрешения при включении egress isolation.
func PatchNetworkPolicies(b []byte) ([]byte, error) {
	d := yaml.NewDecoder(bytes.NewReader(b))
	var docs []M
	var public M
	for {
		var v M
		e := d.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if v == nil {
			continue
		}
		if Map(v["metadata"])["name"] == "proxy-public-egress" {
			public = v
		}
		if Map(v["metadata"])["name"] == "vpc-proxy-gateway-baseline" {
			continue
		}
		docs = append(docs, v)
	}
	if public == nil {
		return nil, errors.New("missing connector public policy")
	}
	encoded, e := yaml.Marshal(public)
	if e != nil {
		return nil, e
	}
	var gateway M
	if e = yaml.Unmarshal(encoded, &gateway); e != nil {
		return nil, e
	}
	gateway["metadata"] = M{"name": "vpc-proxy-gateway-baseline", "namespace": "ate-system"}
	spec := Map(gateway["spec"])
	spec["podSelector"] = M{"matchLabels": M{"app": "atenet-egress"}}
	peer := func(namespace, label, value string) M {
		return M{"namespaceSelector": M{"matchLabels": M{"kubernetes.io/metadata.name": namespace}}, "podSelector": M{"matchLabels": M{label: value}}}
	}
	rule := func(target M, ports ...M) M {
		p := make([]any, len(ports))
		for i, v := range ports {
			p[i] = v
		}
		return M{"to": []any{target}, "ports": p}
	}
	spec["egress"] = append(list(spec["egress"]),
		rule(peer("kube-system", "k8s-app", "kube-dns"), M{"protocol": "UDP", "port": 53}, M{"protocol": "TCP", "port": 53}),
		rule(peer("ate-system", "app", "ate-api-server"), M{"protocol": "TCP", "port": 443}),
		rule(peer("otel-system", "app", "opentelemetry-collector"), M{"protocol": "TCP", "port": 4317}),
	)
	docs = append(docs, gateway)
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	for _, v := range docs {
		if e = enc.Encode(v); e != nil {
			return nil, e
		}
	}
	return out.Bytes(), nil
}
