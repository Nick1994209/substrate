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
for path in map(Path, sys.argv[1:]):
    v = json.loads(path.read_text())
    v.pop('status', None)
    md = v['metadata']
    for key in ('uid', 'resourceVersion', 'managedFields', 'generation', 'creationTimestamp'):
        md.pop(key, None)
    md.get('annotations', {}).pop('kubectl.kubernetes.io/last-applied-configuration', None)
    path.write_text(json.dumps(v, indent=2))
