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
p = w / 'state.json'
state = json.loads(p.read_text())
state['cluster_uid'] = (w / 'cluster-id').read_text().strip()
state['baseline_sha'] = (w / 'baseline.sha').read_text().strip()
state['baseline_objects'] = json.loads((w / 'baseline-objects.json').read_text())
state['rendered'] = {'images': json.loads((w / 'images.json').read_text()),
                     'config_sha256': (w / 'rendered/config.sha256').read_text().strip()}
p.write_text(json.dumps(state, indent=2))
