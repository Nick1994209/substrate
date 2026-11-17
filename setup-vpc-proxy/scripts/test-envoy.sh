#!/usr/bin/env bash

# Copyright 2026 The Agent Substrate Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
owned
export VPC_PROXY_ENVOY_IMAGE
VPC_PROXY_ENVOY_IMAGE="$(k -n ate-system get deployment atenet-egress -o jsonpath='{.spec.template.spec.containers[?(@.name=="envoy")].image}')"
export VPC_PROXY_EVIDENCE="$ARTIFACTS"
go test -tags=vpcproxy_envoy -count=3 -shuffle=on ./cmd/atenet/internal/router/egress -run '^TestVPCProxyEnvoy$' > "$ARTIFACTS/envoy-local.log" 2>&1
