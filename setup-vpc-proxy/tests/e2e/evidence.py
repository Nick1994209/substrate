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

def fetch_matches(data, handler_status, expected):
    origin = data.get('statusCode', 0) or 0
    if expected:
        return handler_status == 200 and 200 <= origin < 400 and bool(data.get('body')) and not data.get('error')
    # Любой ответ origin, включая 500, доказывает достижимость origin.
    return origin == 0 and bool(data.get('error'))


def actor_denied(data, target, gateway_denied):
    if data.get('statusCode') or data.get('tls'):
        return False
    error = data.get('error', '')
    if error.startswith(f'dialing proxy {target}:8443:'):
        return any(reason in error for reason in ('i/o timeout', 'connection refused', 'network is unreachable', 'no route to host'))
    # TLS/protocol rejection сам по себе не доказывает сетевой isolation.
    return (gateway_denied and error.startswith(f'TLS handshake with proxy {target}:8443:')
            and any(reason in error for reason in ('EOF', 'connection reset by peer', 'i/o timeout')))
