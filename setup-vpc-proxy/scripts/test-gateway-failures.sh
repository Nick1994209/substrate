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
gateway_pods() { k -n ate-system get pods -l app=atenet-egress -o name; }
wait_gateway() {
  k -n ate-system rollout status deployment/atenet-egress --timeout=180s
  # rollout может завершиться до остановки старых HTTP/2 sessions.
  for pod in $old_pods; do k -n ate-system wait "$pod" --for=delete --timeout=180s; done
}
k -n ate-system get deployment atenet-egress -o json > "$WORK/gateway-before-test.json"
python3 "$PROJECT/scripts/gateway-args.py" "$WORK"
restore() {
  old_pods="$(gateway_pods)"
  k -n ate-system patch deployment atenet-egress --type=strategic --patch-file="$WORK/restore-args.json"
  wait_gateway
}
trap restore EXIT
old_pods="$(gateway_pods)"
k -n ate-system patch deployment atenet-egress --type=strategic --patch-file="$WORK/broken-extproc.json"
wait_gateway
python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase extproc-outage
restore
trap - EXIT
# Изменённый registry применяется только с rollout, затем восстанавливается.
k -n ate-system get configmap vpc-proxy-routes -o json > "$WORK/registry-before-test.json"
python3 "$PROJECT/scripts/mismatch.py" "$WORK"
restore_registry() {
  old_pods="$(gateway_pods)"
  k -n ate-system apply -f "$WORK/registry-restore.json"
  k -n ate-system rollout restart deployment/atenet-egress
  wait_gateway
}
trap restore_registry EXIT
old_pods="$(gateway_pods)"
k -n ate-system apply -f "$WORK/registry-mismatch.json"
k -n ate-system rollout restart deployment/atenet-egress
wait_gateway
python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase uid-mismatch
