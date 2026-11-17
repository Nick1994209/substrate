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

import ipaddress
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
address = str(ipaddress.ip_address(sys.argv[2]))
daemon = json.loads((work / 'atelet-registry.json').read_text())
aliases = daemon['spec']['template']['spec'].get('hostAliases', [])
aliases = [alias for alias in aliases if 'kind-registry' not in alias['hostnames']]
aliases.append({'ip': address, 'hostnames': ['kind-registry']})
(work / 'registry-host-patch.json').write_text(json.dumps({'spec': {'template': {'spec': {'hostAliases': aliases}}}}))
