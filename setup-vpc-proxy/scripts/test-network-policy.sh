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
"$ROOT/hack/run-tool.sh" ko build ./setup-vpc-proxy/cmd/probe > "$WORK/probe-image.txt"
image="$(cat "$WORK/probe-image.txt")"
[[ "$image" =~ @sha256:[0-9a-f]{64}$ ]]
ns=vpc-proxy-conformance
if k get namespace "$ns" >/dev/null 2>&1; then
  echo 'Conformance namespace already exists; inspect before rerun' >&2
  exit 1
fi
k create namespace "$ns"
cleanup() {
  local status=$?
  if (( status != 0 )); then
    for pod in before denied allowed; do
      if ! k -n "$ns" logs "$pod" > "$ARTIFACTS/cni-$pod.log" 2>&1; then
        echo "Не удалось получить log $pod" >&2
      fi
    done
  fi
  k delete namespace "$ns" --wait=true --timeout=120s
  exit "$status"
}
trap cleanup EXIT
k -n "$ns" create deployment np-server --image="$image" -- /ko-app/probe --serve=:8080
k -n "$ns" patch deployment np-server --type=json -p '[{"op":"add","path":"/spec/template/spec/containers/0/readinessProbe","value":{"httpGet":{"path":"/","port":8080},"periodSeconds":1}}]'
k -n "$ns" rollout status deployment/np-server --timeout=180s
ip="$(k -n "$ns" get pods -l app=np-server -o json | python3 "$PROJECT/scripts/ready-pod-ip.py")"
check() {
  local name="$1"
  local positive=(--wait-http=false)
  if [[ "$name" != denied ]]; then positive=(--wait-http); fi
  k -n "$ns" run "$name" --image="$image" --restart=Never --labels=role=np-client --command -- /ko-app/probe --http "${positive[@]}" --address="$ip:8080"
}
check before
k -n "$ns" wait pod/before --for=jsonpath='{.status.phase}'=Succeeded --timeout=60s
k -n "$ns" apply -f "$PROJECT/tests/e2e/deny.yaml"
check denied
k -n "$ns" wait pod/denied --for=jsonpath='{.status.phase}'=Failed --timeout=60s
k -n "$ns" logs denied | rg 'timeout|deadline exceeded' > "$ARTIFACTS/cni-denied.log"
k -n "$ns" apply -f "$PROJECT/tests/e2e/allow.yaml"
check allowed
k -n "$ns" wait pod/allowed --for=jsonpath='{.status.phase}'=Succeeded --timeout=60s
printf '{"level":"LIVE_KIND","cni":"PASS","control":"allow-deny-allow"}\n' > "$ARTIFACTS/cni.json"
