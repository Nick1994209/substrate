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
pods = [p for p in json.load(sys.stdin)['items']
        if not p['metadata'].get('deletionTimestamp')
        and p.get('status', {}).get('phase') == 'Running'
        and any(c.get('type') == 'Ready' and c.get('status') == 'True'
                for c in p.get('status', {}).get('conditions', []))]
if len(pods) != 1:
    raise SystemExit('expected exactly one ready non-terminating server pod')
ip = ipaddress.ip_address(pods[0]['status']['podIP'])
if ip.version != 4:
    raise SystemExit('expected IPv4 server')
print(ip)
