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


# Каждый запуск начинает новую matrix, без evidence от предыдущего прогона.
for stage in unit preflight validation evidence literal-ip cni smoke mtls down-1 down-2 down-both extproc-outage policy-revoke actor-isolation envoy-san outer-sanitation unknown-actor uid-mismatch repeat rollback redeploy; do
  rm -f "$ARTIFACTS/$stage.json" "$ARTIFACTS/$stage-failure.json"
done
stage=unit
failed() {
  local status=$?
  printf '{"exit":%d}\n' "$status" > "$ARTIFACTS/$stage-failure.json"
  if [[ ! -s "$ARTIFACTS/$stage.json" ]]; then
    printf '{"%s":"FAIL","exit":%d}\n' "$stage" "$status" > "$ARTIFACTS/$stage.json"
  fi
  python3 "$PROJECT/scripts/verification-gate.py" "$ARTIFACTS" || true
  exit "$status"
}
mark() { printf '{"%s":"PASS"}\n' "$stage" > "$ARTIFACTS/$stage.json"; }
trap failed ERR
bash "$PROJECT/scripts/unit.sh"
mark
stage=preflight
bash "$PROJECT/scripts/preflight.sh"
mark
owned
stage=envoy-san
bash "$PROJECT/scripts/test-envoy.sh"
stage=cni
bash "$PROJECT/scripts/test-network-policy.sh"
stage=validation
bash "$PROJECT/scripts/validate.sh"
mark
stage=smoke
bash "$PROJECT/scripts/smoke.sh"
stage=literal-ip
python3 "$PROJECT/tests/e2e/actors.py" --work "$WORK" --evidence "$ARTIFACTS" --phase literal-ip
stage=mtls
bash "$PROJECT/scripts/test-mtls.sh"
stage=actor-isolation
bash "$PROJECT/scripts/test-isolation.sh"
stage=down-1
bash "$PROJECT/scripts/test-fail-closed.sh"
stage=extproc-outage
bash "$PROJECT/scripts/test-gateway-failures.sh"
stage=policy-revoke
bash "$PROJECT/scripts/test-policy.sh"
stage=unknown-actor
bash "$PROJECT/scripts/test-unknown-actor.sh"
stage=repeat
bash "$PROJECT/scripts/test-repeat.sh"
stage=rollback
bash "$PROJECT/scripts/cleanup.sh" rollback
bash "$PROJECT/scripts/smoke.sh" baseline
mark
stage=redeploy
bash "$PROJECT/scripts/deploy.sh"
mark
stage=mtls
bash "$PROJECT/scripts/test-mtls.sh"
stage=evidence
bash "$PROJECT/scripts/collect-evidence.sh"
mark
trap - ERR
python3 "$PROJECT/scripts/verification-gate.py" "$ARTIFACTS"
