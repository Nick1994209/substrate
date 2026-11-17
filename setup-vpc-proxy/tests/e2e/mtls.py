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

import contextlib
import json
import selectors
import socket
import subprocess
import sys
from pathlib import Path

w, evidence = map(Path, sys.argv[1:])
results = []

def connect_count(target):
    logs = subprocess.check_output(['kubectl', '--context', 'kind-substrate-vpc-proxy', '-n', 'vpc-proxy', 'logs', f'deployment/proxy-vpc-pod-{target}'], text=True, timeout=20)
    return logs.count('"msg":"connect_open"')

@contextlib.contextmanager
def forward(target):
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    proc = subprocess.Popen(['kubectl', '--context', 'kind-substrate-vpc-proxy', '-n', 'vpc-proxy', 'port-forward', f'svc/proxy-vpc-pod-{target}', f'{port}:8443'], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        with selectors.DefaultSelector() as sel:
            sel.register(proc.stdout, selectors.EVENT_READ)
            if not sel.select(timeout=20) or 'Forwarding' not in proc.stdout.readline():
                raise RuntimeError('proxy port-forward not ready')
        yield port
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)

for target in (1, 2):
    cases = [(f'client-{client}', f'pki/{client}/client', target == client, False) for client in (1, 2)]
    cases.append(('no certificate', None, False, False))
    if target == 1:
        cases.extend((case, f'test-pki/{case}', False, False) for case in ('wrong-uri', 'expired', 'wrong-eku', 'foreign-ca'))
    cases.append(('wrong server URI', f'pki/{target}/client', False, True))
    for case, client, expected, wrong_server in cases:
        row = {'level': 'LIVE_KIND_PORT_FORWARD', 'target': target, 'case': case}
        try:
            # Каждый TLS отказ получает отдельный port-forward: stream error
            # kubectl может закрыть весь предыдущий forwarding session.
            with forward(target) as port:
                server = 3-target if wrong_server else target
                cmd = [str(w / 'bin/probe'), '--address', f'127.0.0.1:{port}', '--ca', str(w / f'pki/{target}/ca.crt'), '--server-name', f'proxy-vpc-pod-{target}.vpc-proxy.svc.cluster.local', '--server-uri', f'spiffe://vpc-proxy.local/proxy/proxy-{server}']
                if client:
                    cmd.extend(['--cert', str(w / (client + '.crt')), '--key', str(w / (client + '.key'))])
                run = subprocess.run(cmd, capture_output=True, text=True, timeout=12)
                if expected:
                    passed = run.returncode == 0 and 'CONNECT OK' in run.stdout
                else:
                    reason = 'server URI rejected' if wrong_server else 'remote error: tls:'
                    passed = run.returncode != 0 and reason in run.stderr
                row.update(status='PASS' if passed else 'FAIL', exit=run.returncode, output=run.stdout + run.stderr)
        except Exception as error:
            row.update(status='FAIL', harness_error=str(error))
        results.append(row)
    node = json.loads((w / 'nodes.json').read_text())['items'][0]
    node_ip = next(address['address'] for address in node['status']['addresses'] if address['type'] == 'InternalIP')
    for authority in ('10.0.0.1:443', '169.254.169.254:80', '127.0.0.1:443', node_ip + ':443', '10.96.0.1:443', '1.1.1.1:22', 'example.com:443'):
        row = {'level': 'LIVE_KIND_PORT_FORWARD', 'target': target, 'case': 'destination denied', 'authority': authority}
        try:
            with forward(target) as port:
                cmd = [str(w / 'bin/probe'), '--address', f'127.0.0.1:{port}', '--ca', str(w / f'pki/{target}/ca.crt'), '--server-name', f'proxy-vpc-pod-{target}.vpc-proxy.svc.cluster.local', '--server-uri', f'spiffe://vpc-proxy.local/proxy/proxy-{target}', '--cert', str(w / f'pki/{target}/client.crt'), '--key', str(w / f'pki/{target}/client.key'), '--authority', authority]
                run = subprocess.run(cmd, capture_output=True, text=True, timeout=12)
                expected_status = 400 if authority == 'example.com:443' else 403
                row.update(status='PASS' if run.returncode != 0 and f'CONNECT {expected_status}' in run.stderr else 'FAIL', output=run.stdout + run.stderr)
        except Exception as error:
            row.update(status='FAIL', harness_error=str(error))
        results.append(row)
    row = {'level': 'LIVE_KIND_PORT_FORWARD', 'target': target, 'case': 'plaintext CONNECT'}
    try:
        before = connect_count(target)
        with forward(target) as port, socket.create_connection(('127.0.0.1', port), timeout=5) as connection:
            connection.sendall(b'CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\n\r\n')
            try:
                answer = connection.recv(1024)
                denied = not answer or answer.startswith((b'HTTP/1.0 400 ', b'HTTP/1.1 400 '))
                row['response'] = answer.decode(errors='replace')
            except ConnectionResetError:
                denied = True
                row['response'] = 'connection reset'
        delta = connect_count(target) - before
        row.update(status='PASS' if denied and delta == 0 else 'FAIL', proxy_connect_delta=delta)
    except Exception as error:
        row.update(status='FAIL', harness_error=str(error))
    results.append(row)
(evidence / 'mtls.json').write_text(json.dumps(results, indent=2))
print(json.dumps(results, indent=2))
raise SystemExit(0 if results and all(r['status'] == 'PASS' for r in results) else 1)
