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
ate get egress-policy actor-3 -a ate-demo-egress -o json > "$WORK/policy-3.json"
revoked=0
restore() {
  if [[ "$revoked" == 0 ]]; then return; fi
  ate get egress-policy actor-3 -a ate-demo-egress -o json > "$WORK/policy-3-current.json"
  python3 "$PROJECT/scripts/policy-rewrite.py" "$WORK" restore
  ate update egress-policy actor-3 -a ate-demo-egress -f "$WORK/policy-3-restore.json"
}
trap restore EXIT
python3 "$PROJECT/scripts/policy-rewrite.py" "$WORK" deny
ate update egress-policy actor-3 -a ate-demo-egress -f "$WORK/policy-3-deny.json"
revoked=1
deadline=$((SECONDS + 90))
until python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase policy-revoke; do
  if (( SECONDS >= deadline )); then exit 1; fi
done
