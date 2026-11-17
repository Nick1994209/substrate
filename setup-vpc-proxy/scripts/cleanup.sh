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
# Rollback намеренно выключает feature. Он не используется при outage.
old_generation="$(k -n ate-system get deployment atenet-egress -o jsonpath='{.metadata.generation}')"
old_pods="$(k -n ate-system get pods -l app=atenet-egress -o name)"
k apply -f "$WORK/baseline-configmap.yaml"
k apply -f "$WORK/baseline-deployment.yaml"
k -n ate-system rollout status deployment/atenet-egress --timeout=180s
if [[ "$(k -n ate-system get deployment atenet-egress -o jsonpath='{.metadata.generation}')" != "$old_generation" ]]; then
  for pod in $old_pods; do k -n ate-system wait "$pod" --for=delete --timeout=180s; done
fi
k -n ate-system delete configmap vpc-proxy-routes --ignore-not-found
k -n ate-system delete secret vpc-proxy-client-1 vpc-proxy-client-2 --ignore-not-found
k -n ate-system delete networkpolicy egress-to-actor-proxies vpc-proxy-gateway-baseline --ignore-not-found
if k get namespace vpc-proxy >/dev/null 2>&1; then
  test -s "$WORK/proxy-namespace-id"
  test "$(k get namespace vpc-proxy -o jsonpath='{.metadata.uid}')" = "$(cat "$WORK/proxy-namespace-id")"
  test "$(k get namespace vpc-proxy -o jsonpath='{.metadata.labels.app\.kubernetes\.io/part-of}')" = setup-vpc-proxy
  k delete namespace vpc-proxy --wait=true --timeout=180s
fi
if [[ "${1:-rollback}" == cleanup ]]; then
  for id in 1 2 3; do
    ate get actor "actor-$id" -a ate-demo-egress -o json > "$WORK/actor-$id.current.json"
    python3 "$PROJECT/scripts/check-actor.py" "$WORK/actors/actor-$id.json" "$WORK/actor-$id.current.json"
    ate delete actor "actor-$id" -a ate-demo-egress
  done
  # Маркеры actor сохраняются: имена нельзя использовать повторно с живыми SVID.
  if [[ "${DESTROY_CLUSTER:-0}" == 1 ]]; then
    owned
    kind delete cluster --name "$KIND_CLUSTER_NAME"
  fi
fi
