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

import json
import sys
from pathlib import Path
root = Path(sys.argv[1])
required = ['unit', 'preflight', 'validation', 'evidence', 'literal-ip', 'cni', 'smoke', 'mtls', 'down-1', 'down-2', 'down-both', 'extproc-outage', 'policy-revoke', 'actor-isolation', 'envoy-san', 'outer-sanitation', 'unknown-actor', 'uid-mismatch', 'repeat', 'rollback', 'redeploy']
matrix = {}
for stage in required:
    if (root / (stage + '-failure.json')).exists():
        matrix[stage] = 'FAIL'
        continue
    p = root / (stage + '.json')
    if not p.exists():
        matrix[stage] = 'BLOCKED'
        continue
    data = json.loads(p.read_text())
    matrix[stage] = 'PASS' if (isinstance(data, list) and data and all(r.get('status') == 'PASS' for r in data)) or (isinstance(data, dict) and data.get(stage) == 'PASS') else 'FAIL'
(root / 'verify.json').write_text(json.dumps(matrix, indent=2))
print(json.dumps(matrix, indent=2))
raise SystemExit(0 if all(v == 'PASS' for v in matrix.values()) else 1)
