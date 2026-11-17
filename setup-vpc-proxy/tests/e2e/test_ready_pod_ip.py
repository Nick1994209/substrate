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
import subprocess
import sys
import unittest
from pathlib import Path


class ReadyPodIP(unittest.TestCase):
    def test_selection(self):
        script = Path(__file__).resolve().parents[2] / 'scripts/ready-pod-ip.py'
        def pod(ip, deleting=False):
            return {'metadata': {'deletionTimestamp': '2026-11-06T00:00:00Z'} if deleting else {},
                    'status': {'phase': 'Running', 'podIP': ip, 'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        for name, pods, expected in [
            ('terminating_old', [pod('10.0.0.1', True), pod('10.0.0.2')], '10.0.0.2'),
            ('ambiguous', [pod('10.0.0.1'), pod('10.0.0.2')], None),
            ('none', [pod('10.0.0.1', True)], None),
        ]:
            with self.subTest(name=name):
                result = subprocess.run([sys.executable, str(script)], input=json.dumps({'items': pods}), text=True, capture_output=True, timeout=5)
                if expected is None:
                    self.assertNotEqual(result.returncode, 0)
                else:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout.strip(), expected)


if __name__ == '__main__':
    unittest.main()
