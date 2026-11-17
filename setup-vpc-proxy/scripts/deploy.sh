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
bash "$PROJECT/scripts/render.sh"
bash "$PROJECT/scripts/validate.sh"
# Старый gateway останавливается до изменения ConfigMap: нет окна legacy bypass.
k -n ate-system scale deployment/atenet-egress --replicas=0
k -n ate-system wait pod -l app=atenet-egress --for=delete --timeout=180s
ensure_proxy_namespace
k apply -f "$WORK/rendered/networkpolicies.yaml"
k apply -f "$WORK/rendered/secrets.yaml"
k apply -f "$WORK/rendered/proxies.yaml"
for id in 1 2; do k -n vpc-proxy rollout status deployment/"proxy-vpc-pod-$id" --timeout=180s; done
k apply -f "$WORK/rendered/gateway.yaml"
k -n ate-system rollout status deployment/atenet-egress --timeout=180s
bash "$PROJECT/scripts/smoke.sh"
