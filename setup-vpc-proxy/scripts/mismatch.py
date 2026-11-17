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

import copy
import json
import sys
import uuid
from pathlib import Path
w = Path(sys.argv[1])
v = json.loads((w / 'registry-before-test.json').read_text())
v['metadata'] = {k: v['metadata'][k] for k in ('name', 'namespace')}
(w / 'registry-restore.json').write_text(json.dumps(v))
bad = copy.deepcopy(v)
routes = json.loads((w / 'routes.yaml').read_text())
for row in routes['routes']:
    if row['actor']['name'] == 'actor-1':
        row['actor']['uid'] = str(uuid.uuid4())
bad['data']['routes.yaml'] = json.dumps(routes)
(w / 'registry-mismatch.json').write_text(json.dumps(bad))
