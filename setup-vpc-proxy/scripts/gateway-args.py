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
v = json.loads((w / 'gateway-before-test.json').read_text())
c = [c for c in v['spec']['template']['spec']['containers'] if c['name'] == 'ext-proc']
if len(c) != 1 or c[0]['args'].count('--port-extproc=50051') != 1:
    raise SystemExit('unexpected ext-proc arguments')
for broken in (False, True):
    args = [arg.replace('--port-extproc=50051', '--port-extproc=50052') if broken else arg for arg in c[0]['args']]
    patch = {'spec': {'template': {'spec': {'containers': [{'name': 'ext-proc', 'args': args}]}}}}
    (w / ('broken-extproc.json' if broken else 'restore-args.json')).write_text(json.dumps(patch))
