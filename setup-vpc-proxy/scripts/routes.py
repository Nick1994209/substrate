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
w = Path(sys.argv[1])
routes = []
for i in range(1, 4):
    actor = json.loads((w / f'actor-{i}.current.json').read_text())['metadata']
    row = {'actor': {k: actor[k] for k in ('atespace', 'name', 'uid')}, 'mode': 'proxy' if i < 3 else 'direct'}
    if i < 3:
        row['cluster'] = f'vpc_proxy_{i}'
    routes.append(row)
config = {'apiVersion': 'vpc-proxy.substrate.local/v1alpha1', 'kind': 'ActorEgressRoutes', 'routes': routes}
new = json.dumps(config, indent=2) + '\n'
p = w / 'routes.yaml'
if p.exists() and json.loads(p.read_text()) != config:
    raise SystemExit('registry changed: explicit review required')
p.write_text(new)
(w / 'state.json').write_text(json.dumps({'actors': routes}, indent=2))
