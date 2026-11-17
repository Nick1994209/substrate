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
import sys
from pathlib import Path
w = Path(sys.argv[1])
hashes = {name: hashlib.sha256((w / name).read_bytes()).hexdigest()
          for name in ('baseline-configmap.yaml', 'baseline-deployment.yaml')}
p = w / 'baseline-objects.json'
if p.exists() and json.loads(p.read_text()) != hashes:
    raise SystemExit('baseline snapshot hashes changed')
p.write_text(json.dumps(hashes, indent=2))
