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
snapshot() {
  for id in 1 2 3; do
    ate get actor "actor-$id" -a ate-demo-egress -o json > "$WORK/actor-$id.current.json"
  done
  k -n vpc-proxy get deployments -l app.kubernetes.io/part-of=setup-vpc-proxy -o json > "$WORK/repeat-resources.json"
  python3 "$PROJECT/scripts/repeat-state.py" "$WORK" "$1"
}
snapshot "$ARTIFACTS/repeat-before.json"
bash "$PROJECT/scripts/deploy.sh"
snapshot "$ARTIFACTS/repeat-after.json"
cmp "$ARTIFACTS/repeat-before.json" "$ARTIFACTS/repeat-after.json"
printf '{"repeat":"PASS"}\n' > "$ARTIFACTS/repeat.json"
