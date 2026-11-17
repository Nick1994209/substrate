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
restore() {
  for id in 1 2; do k -n vpc-proxy scale deployment/"proxy-vpc-pod-$id" --replicas=1; done
  for id in 1 2; do k -n vpc-proxy rollout status deployment/"proxy-vpc-pod-$id" --timeout=180s; done
}
trap restore EXIT
for down in 1 2 both; do
  restore
  for id in 1 2; do
    if [[ "$down" == "$id" || "$down" == both ]]; then
      k -n vpc-proxy scale deployment/"proxy-vpc-pod-$id" --replicas=0
      k -n vpc-proxy wait pods -l "app=actor-vpc-proxy,proxy-id=$id" --for=delete --timeout=180s
    fi
  done
  python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase "down-$down"
  restore
  python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase "restored-$down"
done
