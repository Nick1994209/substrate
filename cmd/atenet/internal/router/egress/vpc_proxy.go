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
	"log/slog"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	envoytype "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/atenet/internal/router/extproc"
	"go.ate.dev/substrate/internal/egresspolicy"
	"go.ate.dev/substrate/internal/resources"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/bindings"
	"go.ate.dev/substrate/setup-vpc-proxy/pkg/netpolicy"
)

const vpcClusterHeader = "x-ate-vpc-cluster"

type Option func(*Handler)

func WithVPCProxyRegistry(reg *bindings.Registry) Option {
	return func(h *Handler) { h.vpcProxyRegistry = reg }
}

// false продолжает штатный EgressPolicy pipeline.
func (h *Handler) routeVPCProxy(
	ctx context.Context,
	ref resources.ActorRef,
	actor *ateapi.Actor,
	dest egresspolicy.Destination,
) (extproc.Result, bool, error) {
	if h.vpcProxyRegistry == nil {
		return extproc.Result{}, false, nil // Feature выключен.
	}

	deny := func(reason string) (extproc.Result, bool, error) {
		slog.WarnContext(ctx, "vpc_proxy_denied",
			"actor", ref, "reason", reason)
		return extproc.Result{}, true,
			extproc.NewReqError(envoytype.StatusCode_Forbidden, "egress denied")
	}

	uid := actor.GetMetadata().GetUid()
	if uid == "" {
		return deny("missing_actor_uid")
	}
	decision, err := h.vpcProxyRegistry.Resolve(ref.Atespace, ref.Name, uid)
	if err != nil {
		return deny(err.Error())
	}

	switch decision.Mode {
	case bindings.ModeDirect:
		slog.InfoContext(ctx, "vpc_proxy_route", "actor", ref,
			"actor_uid", uid, "mode", "direct", "destination_ip", dest.IP.String(), "destination_port", dest.Port)
		return extproc.Result{}, false, nil

	case bindings.ModeRevoked:
		return deny("binding_revoked")

	case bindings.ModeProxy:
		// Проверки proxy следуют ниже.

	default:
		return deny("unsupported_mode")
	}

	if decision.Cluster != "vpc_proxy_1" && decision.Cluster != "vpc_proxy_2" {
		return deny("unsupported_cluster")
	}
	if dest.Port == 0 {
		return deny("invalid_destination_port")
	}
	if err := netpolicy.CheckPublicTarget(dest.IP.Unmap(), uint16(dest.Port)); err != nil {
		return deny("destination_not_allowed")
	}

	metadata, err := connectMetadata(dest, nil)
	if err != nil {
		return extproc.Result{}, true,
			extproc.NewReqError(envoytype.StatusCode_ServiceUnavailable,
				"egress unavailable")
	}

	res := allow()
	res.Target = decision.Cluster // Диагностика; маршрут меняет HeaderMutation.
	res.DynamicMetadata = metadata
	res.Response.Response.ClearRouteCache = true
	res.Response.Response.HeaderMutation = &extprocv3.HeaderMutation{
		SetHeaders: []*corev3.HeaderValueOption{
			{
				Header: &corev3.HeaderValue{
					Key:      vpcClusterHeader,
					RawValue: []byte(decision.Cluster),
				},
				AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
			},
		},
	}

	slog.InfoContext(ctx, "vpc_proxy_route", "actor", ref,
		"actor_uid", uid, "mode", "proxy", "cluster", decision.Cluster,
		"destination_ip", dest.IP.String(), "destination_port", dest.Port)
	return res, true, nil
}
