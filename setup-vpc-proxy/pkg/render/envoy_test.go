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
	"io"
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func fixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	b, e := os.ReadFile("../../../manifests/ate-install/atenet-egress.yaml")
	if e != nil {
		t.Fatal(e)
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	var cfg []byte
	for {
		var v M
		e = d.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if v["kind"] == "ConfigMap" && Map(v["metadata"])["name"] == "atenet-egress" {
			cfg = []byte(Map(v["data"])["envoy.yaml"].(string))
		}
	}
	p, e := os.ReadFile("../../config/envoy-patch.yaml")
	if e != nil {
		t.Fatal(e)
	}
	if len(cfg) == 0 {
		t.Fatal("missing config")
	}
	return cfg, p
}
func hcm(c M) M {
	l, _ := named(list(Map(c["static_resources"])["listeners"]), "egress")
	ch := Map(list(l["filter_chains"])[0])
	return Map(Map(list(ch["filters"])[0])["typed_config"])
}
func TestPatchEnvoy(t *testing.T) {
	b, p := fixture(t)
	out, e := PatchEnvoy(b, p)
	if e != nil {
		t.Fatal(e)
	}
	again, e := PatchEnvoy(out, p)
	if e != nil || !bytes.Equal(out, again) {
		t.Fatal("not idempotent", e)
	}
	var before, after M
	_ = yaml.Unmarshal(b, &before)
	_ = yaml.Unmarshal(out, &after)
	bs, as := Map(before["static_resources"]), Map(after["static_resources"])
	for _, v := range list(bs["listeners"]) {
		name := Map(v)["name"].(string)
		if name == "egress" {
			continue
		}
		a, e := named(list(as["listeners"]), name)
		if e != nil || !reflect.DeepEqual(v, a) {
			t.Fatal("inner listener changed", name, e)
		}
	}
	for _, name := range []string{"vpc_proxy_1", "vpc_proxy_2"} {
		c, e := named(list(as["clusters"]), name)
		if e != nil {
			t.Fatal(e)
		}
		params := Map(Map(Map(c["transport_socket"])["typed_config"])["common_tls_context"])["tls_params"]
		if Map(params)["tls_minimum_protocol_version"] != "TLSv1_3" || Map(params)["tls_maximum_protocol_version"] != "TLSv1_3" {
			t.Fatal("upstream TLS version range invalid")
		}
	}
	h := hcm(after)
	vh := Map(list(Map(h["route_config"])["virtual_hosts"])[0])
	routes := list(vh["routes"])
	if len(routes) != 4 || Map(routes[2])["direct_response"] == nil {
		t.Fatal("deny route missing")
	}
	for _, v := range routes[:2] {
		u := list(Map(Map(v)["route"])["upgrade_configs"])
		if Map(u[0])["connect_config"] != nil {
			t.Fatal("proxy CONNECT terminated early")
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(M)
	}{
		{"virtual host override", func(c M) {
			vh := Map(list(Map(hcm(c)["route_config"])["virtual_hosts"])[0])
			vh["typed_per_filter_config"] = M{"envoy.filters.http.ext_proc": M{"disabled": true}}
		}},
		{"unknown filter", func(c M) {
			h := hcm(c)
			h["http_filters"] = append([]any{M{"name": "envoy.filters.http.ext_authz"}}, list(h["http_filters"])...)
		}},
		{"fail open", func(c M) {
			ep, _ := named(list(hcm(c)["http_filters"]), "envoy.filters.http.ext_proc")
			Map(ep["typed_config"])["failure_mode_allow"] = true
		}},
		{"missing outer", func(c M) { Map(c["static_resources"])["listeners"] = []any{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c M
			_ = yaml.Unmarshal(b, &c)
			tc.mutate(c)
			x, e := yaml.Marshal(c)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = PatchEnvoy(x, p); e == nil {
				t.Fatal("accepted unsafe baseline")
			}
		})
	}
}
