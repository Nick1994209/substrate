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
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrUnknownActor = errors.New("actor has no explicit egress binding")
	ErrUIDMismatch  = errors.New("actor UID does not match binding")
)

type Decision struct {
	Mode    Mode
	Cluster string
}
type actorName struct{ atespace, name string }
type Registry struct{ routes map[actorName]Route }

func New(cfg FileConfig) (*Registry, error) {
	if cfg.APIVersion != "vpc-proxy.substrate.local/v1alpha1" || cfg.Kind != "ActorEgressRoutes" {
		return nil, errors.New("unsupported registry schema")
	}
	if len(cfg.Routes) != 3 {
		return nil, errors.New("MVP requires exactly three explicit actor records")
	}
	r := &Registry{routes: make(map[actorName]Route)}
	uids := map[string]bool{}
	for _, v := range cfg.Routes {
		if !validName(v.Actor.Atespace) || !validName(v.Actor.Name) {
			return nil, errors.New("invalid actor name or atespace")
		}
		if !uidPattern.MatchString(v.Actor.UID) || v.Actor.UID == "00000000-0000-0000-0000-000000000000" {
			return nil, errors.New("invalid actor UID")
		}

		k := actorName{v.Actor.Atespace, v.Actor.Name}
		if _, ok := r.routes[k]; ok {
			return nil, errors.New("duplicate actor")
		}
		if uids[v.Actor.UID] {
			return nil, errors.New("duplicate UID")
		}
		switch v.Mode {
		case ModeProxy:
			expected := map[string]string{"actor-1": "vpc_proxy_1", "actor-2": "vpc_proxy_2"}[v.Actor.Name]
			if expected == "" || v.Cluster != expected {
				return nil, errors.New("proxy cluster does not match actor binding")
			}
		case ModeDirect:
			if v.Cluster != "" || v.Actor.Name != "actor-3" {
				return nil, errors.New("invalid direct binding")
			}
		case ModeRevoked:
			if v.Cluster != "" {
				return nil, errors.New("revoked binding has cluster")
			}
		default:
			return nil, fmt.Errorf("unknown mode %q", v.Mode)
		}
		uids[v.Actor.UID] = true
		r.routes[k] = v
	}
	return r, nil
}
func (r *Registry) Resolve(atespace, name, uid string) (Decision, error) {
	v, ok := r.routes[actorName{atespace, name}]
	if !ok {
		return Decision{}, ErrUnknownActor
	}
	if uid == "" || uid != v.Actor.UID {
		return Decision{}, ErrUIDMismatch
	}
	return Decision{v.Mode, v.Cluster}, nil
}

var uidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var namePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func validName(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) > 63 || !namePattern.MatchString(label) {
			return false
		}
	}
	return true
}
