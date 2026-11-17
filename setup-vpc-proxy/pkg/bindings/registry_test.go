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

package bindings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func valid() FileConfig {
	return FileConfig{APIVersion: "vpc-proxy.substrate.local/v1alpha1", Kind: "ActorEgressRoutes", Routes: []Route{
		{ActorKey{"demo", "actor-1", "11111111-1111-4111-8111-111111111111"}, ModeProxy, "vpc_proxy_1"},
		{ActorKey{"demo", "actor-2", "22222222-2222-4222-8222-222222222222"}, ModeProxy, "vpc_proxy_2"},
		{ActorKey{"demo", "actor-3", "33333333-3333-4333-8333-333333333333"}, ModeDirect, ""},
	}}
}
func TestRegistry(t *testing.T) {
	r, err := New(valid())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range valid().Routes {
		d, e := r.Resolve(v.Actor.Atespace, v.Actor.Name, v.Actor.UID)
		if e != nil || d.Mode != v.Mode || d.Cluster != v.Cluster {
			t.Fatal(d, e)
		}
	}
	if _, e := r.Resolve("demo", "other", "x"); !errors.Is(e, ErrUnknownActor) {
		t.Fatal(e)
	}
	if _, e := r.Resolve("demo", "actor-1", ""); !errors.Is(e, ErrUIDMismatch) {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		change func(*FileConfig)
	}{
		{"duplicate actor", func(c *FileConfig) { c.Routes[1].Actor = c.Routes[0].Actor }},
		{"duplicate uid", func(c *FileConfig) { c.Routes[1].Actor.UID = c.Routes[0].Actor.UID }},
		{"uppercase uid", func(c *FileConfig) { c.Routes[0].Actor.UID = "AAAAAAAA-1111-4111-8111-111111111111" }},
		{"placeholder", func(c *FileConfig) { c.Routes[0].Actor.UID = "REPLACE_WITH_UID" }},
		{"wrong cluster", func(c *FileConfig) { c.Routes[0].Cluster = "vpc_proxy_2" }},
		{"direct cluster", func(c *FileConfig) { c.Routes[2].Cluster = "vpc_proxy_1" }},
		{"unknown mode", func(c *FileConfig) { c.Routes[0].Mode = "fallback" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.change(&c)
			if _, e := New(c); e == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	c := valid()
	c.Routes[0].Mode = ModeRevoked
	c.Routes[0].Cluster = ""
	r, err = New(c)
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.Resolve("demo", "actor-1", c.Routes[0].Actor.UID)
	if err != nil || d.Mode != ModeRevoked {
		t.Fatal(d, err)
	}
}
func TestStrictLoad(t *testing.T) {
	for _, data := range []string{"unknown: true", "kind: ActorEgressRoutes\nkind: ActorEgressRoutes", `{"kind":"ActorEgressRoutes","kind":"ActorEgressRoutes"}`, "---\n{}\n---\n{}", strings.Repeat("x", maxConfigSize+1)} {
		p := filepath.Join(t.TempDir(), "routes.yaml")
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, e := LoadFile(p); e == nil {
			t.Fatal("accepted invalid config")
		}
	}
}
