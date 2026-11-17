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

package egress

import (
	"context"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/atenet/internal/router/extproc"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/bindings"
	"testing"
)

func TestVPCProxyAuthenticatedConnect(t *testing.T) {
	ca := newTestCA(t, "actor-ca")
	for _, tc := range []struct {
		name, cluster string
		mode          bindings.Mode
		uid           string
		state         ateapi.ActorState
		cert          bool
		allow         bool
	}{
		{"actor-1", "vpc_proxy_1", bindings.ModeProxy, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, true, true},
		{"actor-2", "vpc_proxy_2", bindings.ModeProxy, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, true, true},
		{"actor-3", "", bindings.ModeDirect, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, true, false},
		{"actor-1", "", bindings.ModeRevoked, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, true, false},
		{"actor-1", "vpc_proxy_1", bindings.ModeProxy, "44444444-4444-4444-8444-444444444444", ateapi.ActorState_ACTOR_STATE_RUNNING, true, false},
		{"actor-1", "vpc_proxy_1", bindings.ModeProxy, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_SUSPENDED, true, false},
		{"actor-1", "vpc_proxy_1", bindings.ModeProxy, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, false, false},
		{"unknown", "", bindings.ModeDirect, testEgressActorUID, ateapi.ActorState_ACTOR_STATE_RUNNING, true, false},
	} {
		t.Run(tc.name+string(tc.mode)+tc.uid+tc.state.String(), func(t *testing.T) {
			routes := []bindings.Route{{Actor: bindings.ActorKey{Atespace: "default", Name: "actor-1", UID: testEgressActorUID}, Mode: bindings.ModeProxy, Cluster: "vpc_proxy_1"}, {Actor: bindings.ActorKey{Atespace: "default", Name: "actor-2", UID: "22222222-2222-4222-8222-222222222222"}, Mode: bindings.ModeProxy, Cluster: "vpc_proxy_2"}, {Actor: bindings.ActorKey{Atespace: "default", Name: "actor-3", UID: "33333333-3333-4333-8333-333333333333"}, Mode: bindings.ModeDirect}}
			if tc.name != "actor-1" {
				routes[0].Actor.UID = "11111111-1111-4111-8111-111111111111"
			}
			for i := range routes {
				if routes[i].Actor.Name == tc.name {
					routes[i].Mode = tc.mode
					routes[i].Cluster = tc.cluster
					routes[i].Actor.UID = tc.uid
				}
			}
			reg, e := bindings.New(bindings.FileConfig{APIVersion: "vpc-proxy.substrate.local/v1alpha1", Kind: "ActorEgressRoutes", Routes: routes})
			if e != nil {
				t.Fatal(e)
			}
			actor := runningActor()
			actor.Metadata.Name = tc.name
			actor.Status.State = tc.state
			mock := &egressMockClient{actor: actor}
			h := New(mock, ca.loadRoots, 0, nil, WithVPCProxyRegistry(reg))
			xfcc := ""
			if tc.cert {
				xfcc = xfccHeader(ca.issueActorCert(t, "spiffe://substrate-actor.local/worker-for-actor/default/"+tc.name, actorCertOptions{}))
			}
			md := egressMetadata(xfcc)
			// Заголовок другого proxy не является identity или binding input.
			if tc.name == "actor-1" {
				md = extproc.NewRequestMetadata([]*corev3.HeaderValue{{Key: ":method", RawValue: []byte("CONNECT")}, {Key: ":authority", RawValue: []byte("1.1.1.1:443")}, {Key: forwardedClientCertHeader, RawValue: []byte(xfcc)}, {Key: "x-ate-vpc-cluster", RawValue: []byte("vpc_proxy_2")}}, nil)
			}
			res, e := h.HandleRequestHeaders(context.Background(), md)
			if !tc.allow {
				if e == nil {
					t.Fatal("unexpected allow")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			cr := res.Response.Response
			if !cr.ClearRouteCache || string(cr.HeaderMutation.SetHeaders[0].Header.RawValue) != tc.cluster {
				t.Fatal("wrong cluster mutation", res)
			}
			if mock.policyCalls.Load() != 0 {
				t.Fatal("proxy consulted direct policy")
			}
		})
	}
}
