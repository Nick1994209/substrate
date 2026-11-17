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
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestGatewayNetworkDependencies(t *testing.T) {
	b, e := os.ReadFile("../../manifests/networkpolicies.yaml.tmpl")
	if e != nil {
		t.Fatal(e)
	}
	out, e := PatchNetworkPolicies(b)
	if e != nil {
		t.Fatal(e)
	}
	again, e := PatchNetworkPolicies(out)
	if e != nil || !bytes.Equal(out, again) {
		t.Fatal("not idempotent", e)
	}
	d := yaml.NewDecoder(bytes.NewReader(out))
	var connector, gateway M
	for {
		var v M
		e = d.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		switch Map(v["metadata"])["name"] {
		case "proxy-public-egress":
			connector = v
		case "vpc-proxy-gateway-baseline":
			gateway = v
		}
	}
	cr := list(Map(connector["spec"])["egress"])
	gr := list(Map(gateway["spec"])["egress"])
	if len(cr) != 1 || len(gr) != 4 || !reflect.DeepEqual(cr[0], gr[0]) {
		t.Fatal("gateway lost public target restrictions or system dependencies")
	}
	for _, v := range gr[1:] {
		if Map(list(Map(v)["to"])[0])["ipBlock"] != nil {
			t.Fatal("system dependency opens CIDR")
		}
	}
}
