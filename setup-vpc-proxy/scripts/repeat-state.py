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

import hashlib
import json
import ssl
import sys
from pathlib import Path

work, output = map(Path, sys.argv[1:])
state = {'actors': [], 'certificates': {}, 'deployments': {}}
for i in (1, 2, 3):
    state['actors'].append(json.loads((work / f'actor-{i}.current.json').read_text())['metadata']['uid'])
for i in (1, 2):
    for role in ('ca', 'client', 'server'):
        cert = (work / f'pki/{i}/{role}.crt').read_text()
        state['certificates'][f'{i}/{role}'] = hashlib.sha256(ssl.PEM_cert_to_DER_cert(cert)).hexdigest()
for item in json.loads((work / 'repeat-resources.json').read_text())['items']:
    state['deployments'][item['metadata']['name']] = item['metadata']['uid']
if set(state['deployments']) != {'proxy-vpc-pod-1', 'proxy-vpc-pod-2'}:
    raise SystemExit('expected exactly two connector deployments')
state['config_sha256'] = (work / 'rendered/config.sha256').read_text().strip()
output.write_text(json.dumps(state, indent=2))
