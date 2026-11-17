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
w = Path(sys.argv[1])
nodes = json.loads((w / 'nodes.json').read_text())['items']
cidrs = set()
for node in nodes:
    cidrs.update(node['spec']['podCIDRs'])
    for a in node['status']['addresses']:
        if a['type'] == 'InternalIP':
            cidrs.add(str(ipaddress.ip_network(a['address'] + '/32')))
for item in json.loads((w / 'servicecidrs.json').read_text())['items']:
    cidrs.update(item['spec']['cidrs'])
for network in json.loads((w / 'docker-network.json').read_text()):
    cidrs.update(row['Subnet'] for row in network['IPAM']['Config'])
text = (w.parent / 'manifests/networkpolicies.yaml.tmpl').read_text()
for cidr in sorted(cidrs):
    net = ipaddress.ip_network(cidr)
    if net.version != 4:
        continue  # IPv6 не разрешён ни Go policy, ни ipBlock 0.0.0.0/0.
    line = '              - ' + str(net)
    if line not in text:
        text = text.replace('      ports:\n        - protocol: TCP\n          port: 80', line + '\n      ports:\n        - protocol: TCP\n          port: 80')
(w / 'networkpolicies.yaml').write_text(text)
(w / 'network-cidrs.json').write_text(json.dumps(sorted(cidrs)))
