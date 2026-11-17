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
for id in 1 2 3; do
  ate get actor "actor-$id" -a ate-demo-egress -o json > "$WORK/actor-$id.current.json"
  python3 "$PROJECT/scripts/check-actor.py" "$WORK/actors/actor-$id.json" "$WORK/actor-$id.current.json"
done
test "$(cat "$WORK/baseline.sha")" = "$(git rev-parse HEAD)"
k get nodes -o json > "$WORK/nodes.json"
k get servicecidrs -o json > "$WORK/servicecidrs.json"
docker network inspect kind > "$WORK/docker-network.json"
python3 "$PROJECT/scripts/network-cidrs.py" "$WORK"
python3 "$PROJECT/scripts/baseline-hashes.py" "$WORK"
go run ./setup-vpc-proxy/cmd/render --work "$WORK"

python3 "$PROJECT/scripts/render-state.py" "$WORK"
