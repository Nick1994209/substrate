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
test "$(cat "$WORK/baseline.sha")" = "$(git rev-parse HEAD)"
mkdir -p "$WORK/actors"
for id in 1 2 3; do
  if [[ ! -e "$WORK/actors/actor-$id.json" ]]; then
    # Создание завершается ошибкой при конфликте имени. Чужие actors не присваиваем.
    ate create actor "actor-$id" -a ate-demo-egress --template egress -o json > "$WORK/actors/actor-$id.json.tmp"
    mv "$WORK/actors/actor-$id.json.tmp" "$WORK/actors/actor-$id.json"
  fi
  ate get actor "actor-$id" -a ate-demo-egress -o json > "$WORK/actor-$id.current.json"
  python3 "$PROJECT/scripts/check-actor.py" "$WORK/actors/actor-$id.json" "$WORK/actor-$id.current.json"
  ate get egress-policy "actor-$id" -a ate-demo-egress -o json > "$WORK/policy-$id.current.json"
  # CLI возвращает 0 и пустой stdout, если policy ещё не существует.
  if [[ ! -s "$WORK/policy-$id.current.json" ]]; then
    ate create egress-policy "actor-$id" -a ate-demo-egress -f "$PROJECT/manifests/actor-policy.yaml.tmpl"
  fi
  state="$(python3 "$PROJECT/scripts/actor-state.py" "$WORK/actor-$id.current.json")"
  if [[ "$state" != ACTOR_STATE_RUNNING && "$state" != ACTOR_STATE_RESUMING ]]; then
    ate resume actor "actor-$id" -a ate-demo-egress
  fi
 done
python3 "$PROJECT/scripts/routes.py" "$WORK"
