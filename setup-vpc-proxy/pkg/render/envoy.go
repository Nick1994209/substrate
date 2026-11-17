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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type M = map[string]any

func Map(v any) M      { m, _ := v.(map[string]any); return m }
func list(v any) []any { a, _ := v.([]any); return a }
func named(a []any, name string) (M, error) {
	var found M
	for _, v := range a {
		m := Map(v)
		if m["name"] == name {
			if found != nil {
				return nil, fmt.Errorf("duplicate %s", name)
			}
			found = m
		}
	}
	if found == nil {
		return nil, fmt.Errorf("missing %s", name)
	}
	return found, nil
}
func Hash(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func PatchEnvoy(b, patch []byte) ([]byte, error) {
	var cfg, p M
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(patch, &p); err != nil {
		return nil, err
	}
	sr := Map(cfg["static_resources"])
	listener, err := named(list(sr["listeners"]), "egress")
	if err != nil {
		return nil, err
	}
	chains := list(listener["filter_chains"])
	if len(chains) != 1 || Map(chains[0])["name"] != "egress" {
		return nil, errors.New("unexpected outer filter chains")
	}
	chain := Map(chains[0])
	tls := Map(Map(chain["transport_socket"])["typed_config"])
	if tls["require_client_certificate"] != true {
		return nil, errors.New("outer mTLS required")
	}
	filters := list(chain["filters"])
	if len(filters) != 1 {
		return nil, errors.New("unexpected outer network filters")
	}
	nf, err := named(filters, "envoy.filters.network.http_connection_manager")
	if err != nil {
		return nil, err
	}
	hcm := Map(nf["typed_config"])
	if hcm["forward_client_cert_details"] != "SANITIZE_SET" || Map(hcm["set_current_client_cert_details"])["chain"] != true {
		return nil, errors.New("XFCC sanitation/chain required")
	}
	rc := Map(hcm["route_config"])
	hosts := list(rc["virtual_hosts"])
	if len(hosts) != 1 {
		return nil, errors.New("unexpected virtual hosts")
	}
	vh := Map(hosts[0])
	if rc["typed_per_filter_config"] != nil || vh["typed_per_filter_config"] != nil {
		return nil, errors.New("per-route overrides are not supported")
	}
	var names []string
	for _, v := range list(hcm["http_filters"]) {
		n, _ := Map(v)["name"].(string)
		if n != "envoy.filters.http.lua" {
			names = append(names, n)
		}
	}
	expected := []string{"envoy.filters.http.set_filter_state", "envoy.filters.http.ext_proc", "envoy.filters.http.dynamic_modules", "envoy.filters.http.set_filter_state", "envoy.filters.http.router"}
	if strings.Join(names, ",") != strings.Join(expected, ",") {
		return nil, errors.New("unexpected outer HTTP filter pipeline")
	}

	var legacy []any
	for _, v := range list(vh["routes"]) {
		name, _ := Map(v)["name"].(string)
		if strings.HasPrefix(name, "actor_proxy_") || name == "reject_unrecognized_proxy_binding" {
			continue
		}
		legacy = append(legacy, v)
	}
	if len(legacy) != 1 || Map(Map(legacy[0])["route"])["cluster"] != "mitm_internal" || Map(legacy[0])["typed_per_filter_config"] != nil {
		return nil, errors.New("unexpected legacy CONNECT route")
	}
	match := Map(Map(legacy[0])["match"])
	if match["connect_matcher"] == nil {
		return nil, errors.New("legacy CONNECT match missing")
	}
	up := list(Map(Map(legacy[0])["route"])["upgrade_configs"])
	if len(up) != 1 || Map(up[0])["connect_config"] == nil {
		return nil, errors.New("legacy CONNECT termination missing")
	}
	routes := list(p["proxy_routes"])
	if len(routes) != 3 {
		return nil, errors.New("expected two proxy routes and deny")
	}
	vh["routes"] = append(routes, legacy...)
	var httpFilters []any
	count := 0
	for _, v := range list(hcm["http_filters"]) {
		m := Map(v)
		if m["name"] == "envoy.filters.http.lua" {
			if Map(m["typed_config"])["inline_code"] != Map(Map(p["sanitation_filter_before_ext_proc"])["typed_config"])["inline_code"] {
				return nil, errors.New("unrecognized Lua filter")
			}
			continue
		}
		if m["name"] == "envoy.filters.http.ext_proc" {
			count++
			ep := Map(m["typed_config"])
			if ep["failure_mode_allow"] != false {
				return nil, errors.New("ext_proc must fail closed")
			}
			if a := ep["route_cache_action"]; a != nil && a != "DEFAULT" {
				return nil, errors.New("unexpected route cache action")
			}
			ep["mutation_rules"] = M{"disallow_all": true, "disallow_system": true, "disallow_is_error": true, "allow_expression": M{"google_re2": M{}, "regex": "^x-ate-vpc-cluster$"}}
			httpFilters = append(httpFilters, p["sanitation_filter_before_ext_proc"])
		}
		if m["typed_per_filter_config"] != nil {
			return nil, errors.New("unexpected per-filter config")
		}
		httpFilters = append(httpFilters, v)
	}
	if count != 1 {
		return nil, errors.New("expected exactly one outer ext_proc")
	}
	hcm["http_filters"] = httpFilters
	var clusters []any
	for _, v := range list(sr["clusters"]) {
		n, _ := Map(v)["name"].(string)
		if strings.HasPrefix(n, "vpc_proxy_") {
			continue
		}
		clusters = append(clusters, v)
	}
	pc := list(p["proxy_clusters"])
	if len(pc) != 2 {
		return nil, errors.New("expected two clusters")
	}
	sr["clusters"] = append(clusters, pc...)
	return yaml.Marshal(cfg)
}
func StripRuntime(obj M) {
	delete(obj, "status")
	md := Map(obj["metadata"])
	for _, k := range []string{"resourceVersion", "uid", "managedFields", "creationTimestamp", "generation"} {
		delete(md, k)
	}
	ann := Map(md["annotations"])
	delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
}
func PatchDeployment(dep M, image, hash string) error {
	if err := CheckImage(image); err != nil {
		return err
	}
	spec := Map(Map(dep["spec"])["template"])
	ps := Map(spec["spec"])
	containers := list(ps["containers"])
	envoy, err := named(containers, "envoy")
	if err != nil {
		return err
	}
	ext, err := named(containers, "ext-proc")
	if err != nil {
		return err
	}
	ext["image"] = image
	var args []any
	for _, v := range list(ext["args"]) {
		if !strings.HasPrefix(fmt.Sprint(v), "--vpc-proxy-config") {
			args = append(args, v)
		}
	}
	ext["args"] = append(args, "--vpc-proxy-config=/etc/vpc-proxy/routes.yaml")
	mount := func(c M, name, path string) {
		a := list(c["volumeMounts"])
		var out []any
		for _, v := range a {
			if Map(v)["name"] != name {
				out = append(out, v)
			}
		}
		c["volumeMounts"] = append(out, M{"name": name, "mountPath": path, "readOnly": true})
	}
	var volumes []any
	for _, v := range list(ps["volumes"]) {
		n, _ := Map(v)["name"].(string)
		if !strings.HasPrefix(n, "vpc-proxy-") {
			volumes = append(volumes, v)
		}
	}
	for _, id := range []string{"1", "2"} {
		name := "vpc-proxy-client-" + id
		mount(envoy, name, "/etc/vpc-proxy/client-"+id)
		volumes = append(volumes, M{"name": name, "secret": M{"secretName": name, "defaultMode": 288}})
	}
	mount(ext, "vpc-proxy-routes", "/etc/vpc-proxy")
	ps["volumes"] = append(volumes, M{"name": "vpc-proxy-routes", "configMap": M{"name": "vpc-proxy-routes"}})
	md := Map(spec["metadata"])
	ann := Map(md["annotations"])
	if ann == nil {
		ann = M{}
		md["annotations"] = ann
	}
	ann["vpc-proxy.substrate.local/config-hash"] = hash
	return nil
}

var imagePattern = regexp.MustCompile(`^[^\s@]+@sha256:[0-9a-f]{64}$`)

func CheckImage(image string) error {
	if !imagePattern.MatchString(image) {
		return errors.New("image must have a canonical sha256 digest")
	}
	return nil
}
