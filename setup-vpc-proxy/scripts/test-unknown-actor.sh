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
# Новый fixture получает уникальное имя при каждом прогоне.
fixture="negative-$(python3 -c 'import uuid; print(uuid.uuid4().hex)')"
mkdir -p "$WORK/negative-actors"
ate create actor "$fixture" -a ate-demo-egress --template egress -o json > "$WORK/negative-actors/$fixture.json"
cleanup() { ate delete actor "$fixture" -a ate-demo-egress; }
trap cleanup EXIT
ate create egress-policy "$fixture" -a ate-demo-egress -f "$PROJECT/manifests/actor-policy.yaml.tmpl"
ate resume actor "$fixture" -a ate-demo-egress
python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase unknown-actor --negative-actor "$fixture"
