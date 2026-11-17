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
if [[ -f "$WORK/baseline.sha" ]]; then
  test "$(cat "$WORK/baseline.sha")" = "$(git rev-parse HEAD)"
  k -n ate-system rollout status deployment/atenet-egress --timeout=180s
  if [[ ! -e "$WORK/baseline-ready" ]]; then
    bash "$PROJECT/scripts/create-actors.sh"
    bash "$PROJECT/scripts/smoke.sh" baseline
    touch "$WORK/baseline-ready"
  fi
  exit 0
fi
go run ./cmd/ate-setup deploy ate-system --help > "$ARTIFACTS/install-help.txt"
go run ./cmd/ate-setup deploy ate-system --no-dev-env --context "$KUBECTL_CONTEXT" \
  --sandbox-classes=gvisor --credential-provider='{"enabled":false}' \
  --ingress-allowed-spiffe-ids=spiffe://cluster.local/ns/ate-system/sa/atenet-client \
  --rollout-timeout=10m 2>&1 | tee "$ARTIFACTS/install.log"
# Upstream atelet ожидает kind-registry, наш registry имеет отдельное имя.
k -n ate-system get daemonset atelet -o json > "$WORK/atelet-registry.json"
registry_ip="$(docker inspect -f '{{.NetworkSettings.Networks.kind.IPAddress}}' substrate-vpc-proxy-registry)"
python3 "$PROJECT/scripts/registry-host.py" "$WORK" "$registry_ip"
k -n ate-system patch daemonset atelet --type=merge --patch-file="$WORK/registry-host-patch.json"
k -n ate-system rollout status daemonset/atelet --timeout=180s
demo=(go run ./cmd/ate-setup deploy demo egress --no-dev-env --context "$KUBECTL_CONTEXT" --rollout-timeout=10m)
if "${demo[@]}" 2>&1 | tee "$ARTIFACTS/demo.log"; then
  :
else
  # Upstream demo ограничивает первый worker rollout пятью минутами.
  # После cold pull timeout ждём этот же Deployment и повторяем demo один раз.
  rg -q 'waiting for deployment/egress in ate-demo-egress:.*context deadline' "$ARTIFACTS/demo.log"
  k -n ate-demo-egress rollout status deployment/egress --timeout=20m
  "${demo[@]}" 2>&1 | tee -a "$ARTIFACTS/demo.log"
fi
mkdir -p "$WORK/bin"
go build -o "$WORK/bin/kubectl-ate" ./cmd/kubectl-ate
k -n ate-demo-egress patch workerpool egress --type=merge -p '{"spec":{"replicas":3}}'
k -n ate-demo-egress get workerpool egress -o yaml > "$ARTIFACTS/workerpool.yaml"
# Исходные runtime объекты сохраняются один раз, до overlay.
k -n ate-system get configmap atenet-egress -o json > "$WORK/baseline-configmap.yaml"
k -n ate-system get deployment atenet-egress -o json > "$WORK/baseline-deployment.yaml"
python3 "$PROJECT/scripts/sanitize-objects.py" "$WORK/baseline-configmap.yaml" "$WORK/baseline-deployment.yaml"
git rev-parse HEAD > "$WORK/baseline.sha"
bash "$PROJECT/scripts/create-actors.sh"
bash "$PROJECT/scripts/smoke.sh" baseline

touch "$WORK/baseline-ready"
