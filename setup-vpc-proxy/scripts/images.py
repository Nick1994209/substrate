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
import re
import sys
from pathlib import Path
w = Path(sys.argv[1])
images = {k: (w / f'{k}-image.txt').read_text().strip() for k in ('gateway', 'proxy')}
if not all(re.fullmatch(r'[^\s]+@sha256:[0-9a-f]{64}', v) for v in images.values()):
    raise SystemExit('expected immutable ko image references')
(w / 'images.json').write_text(json.dumps(images, indent=2))
