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
import tempfile
import unittest
from pathlib import Path


class VerificationGateTest(unittest.TestCase):
    def test_stage_error_overrides_partial_pass(self):
        script = Path(__file__).resolve().parents[2] / 'scripts/verification-gate.py'
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'actor-isolation.json').write_text(json.dumps([{'status': 'PASS'}]))
            (root / 'actor-isolation-failure.json').write_text(json.dumps({'exit': 1}))
            run = subprocess.run([sys.executable, str(script), directory], capture_output=True, text=True)
            self.assertNotEqual(run.returncode, 0)
            matrix = json.loads((root / 'verify.json').read_text())
            self.assertEqual(matrix['actor-isolation'], 'FAIL')
            self.assertEqual(matrix['smoke'], 'BLOCKED')
