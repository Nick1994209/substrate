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
ensure_proxy_namespace
go build -o "$WORK/bin/atenet" ./cmd/atenet
go build -o "$WORK/bin/proxy" ./setup-vpc-proxy/cmd/proxy
# Полный runtime ConfigMap проверяет отдельный Pod с тем же Envoy image.
k -n ate-system get deployment atenet-egress -o json > "$WORK/current-gateway.json"
cleanup_validation() {
  k -n ate-system delete pod vpc-proxy-validation --ignore-not-found --wait=true --timeout=60s
  k -n ate-system delete configmap vpc-proxy-validation --ignore-not-found
  k -n ate-system delete service vpc-proxy-validation --ignore-not-found
  k -n ate-system delete secret vpc-proxy-validation-client-1 vpc-proxy-validation-client-2 --ignore-not-found
}
trap cleanup_validation EXIT
k apply -f "$WORK/rendered/validation.yaml"
k -n ate-system wait pod/vpc-proxy-validation --for=jsonpath='{.status.phase}'=Succeeded --timeout=150s
k -n ate-system logs vpc-proxy-validation > "$ARTIFACTS/envoy-validation.log"
k apply --dry-run=server -f "$WORK/rendered/networkpolicies.yaml"
k apply --dry-run=server -f "$WORK/rendered/proxies.yaml"
k apply --dry-run=server -f "$WORK/rendered/gateway.yaml"
