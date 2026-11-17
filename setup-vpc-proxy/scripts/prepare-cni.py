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
source, lock, dest = map(Path, sys.argv[1:])
s = source.read_text()
images = json.loads(lock.read_text())
actual = set(re.findall(r"image: (\S+)", s))
if actual != set(images):
    raise SystemExit("unexpected Calico images")
for image, digest in images.items():
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise SystemExit("invalid image digest")
    s = s.replace("image: " + image, "image: " + image + "@" + digest)
s = s.replace('# - name: CALICO_IPV4POOL_CIDR', '- name: CALICO_IPV4POOL_CIDR')
s = s.replace('#   value: "192.168.0.0/16"', '  value: "10.244.0.0/16"')
if 'value: "10.244.0.0/16"' not in s:
    raise SystemExit("unexpected Calico CIDR setting")
dest.write_text(s)
