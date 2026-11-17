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
import uuid
from pathlib import Path
old, current = [json.loads(Path(p).read_text()) for p in sys.argv[1:]]
a, b = old['metadata'], current['metadata']
if a['uid'] != b['uid'] or a['name'] != b['name'] or a['atespace'] != b['atespace']:
    raise SystemExit('actor identity changed; do not reuse names')
if str(uuid.UUID(b['uid'])) != b['uid']:
    raise SystemExit('invalid server UID')
