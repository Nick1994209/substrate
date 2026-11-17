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

import unittest
from evidence import actor_denied, fetch_matches


class EvidenceTest(unittest.TestCase):
    def test_origin_error_is_not_network_denial(self):
        self.assertFalse(fetch_matches({'statusCode': 500, 'error': 'origin failed'}, 200, False))
        self.assertTrue(fetch_matches({'error': 'connection reset'}, 502, False))

    def test_reachable_tls_server_is_not_isolation(self):
        target = '10.96.0.8'
        for data in ({'statusCode': 400}, {'error': f'TLS handshake with proxy {target}:8443: remote error: tls: certificate required'}, {'error': 'reading the answer to CONNECT 1.1.1.1:443: EOF'}):
            self.assertFalse(actor_denied(data, target, True))
        error = {'error': f'TLS handshake with proxy {target}:8443: EOF'}
        self.assertFalse(actor_denied(error, target, False))
        self.assertTrue(actor_denied(error, target, True))
