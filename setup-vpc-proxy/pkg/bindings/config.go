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
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeProxy   Mode = "proxy"
	ModeDirect  Mode = "direct"
	ModeRevoked Mode = "revoked"
)

type ActorKey struct {
	Atespace string `json:"atespace" yaml:"atespace"`
	Name     string `json:"name" yaml:"name"`
	UID      string `json:"uid" yaml:"uid"`
}
type Route struct {
	Actor   ActorKey `json:"actor" yaml:"actor"`
	Mode    Mode     `json:"mode" yaml:"mode"`
	Cluster string   `json:"cluster,omitempty" yaml:"cluster,omitempty"`
}
type FileConfig struct {
	APIVersion string  `json:"apiVersion" yaml:"apiVersion"`
	Kind       string  `json:"kind" yaml:"kind"`
	Routes     []Route `json:"routes" yaml:"routes"`
}

const maxConfigSize = 1 << 20

func LoadFile(path string) (*Registry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxConfigSize {
		return nil, fmt.Errorf("registry exceeds %d bytes", maxConfigSize)
	}
	d := yaml.NewDecoder(strings.NewReader(string(b)))
	d.KnownFields(true)
	var cfg FileConfig
	if err = d.Decode(&cfg); err != nil {
		return nil, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("expected exactly one document: %v", err)
	}
	return New(cfg)
}
