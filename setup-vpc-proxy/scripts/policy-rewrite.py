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

work = Path(sys.argv[1])
original = json.loads((work / 'policy-3.json').read_text())
if sys.argv[2] == 'deny':
    policy = original
    policy['rules'] = []
    destination = 'policy-3-deny.json'
else:
    policy = json.loads((work / 'policy-3-current.json').read_text())
    if policy['metadata']['uid'] != original['metadata']['uid']:
        raise SystemExit('policy identity changed')
    policy['rules'] = original.get('rules', [])
    destination = 'policy-3-restore.json'
(work / destination).write_text(json.dumps(policy))
