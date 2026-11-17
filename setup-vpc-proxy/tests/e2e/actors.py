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

import argparse
import concurrent.futures
import contextlib
import datetime
import json
import selectors
import socket
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path
from evidence import actor_denied, fetch_matches

p = argparse.ArgumentParser()
p.add_argument('--work', type=Path, required=True)
p.add_argument('--evidence', type=Path, required=True)
p.add_argument('--phase', default='smoke')
p.add_argument('--negative-actor')
a = p.parse_args()
context = 'kind-substrate-vpc-proxy'
cli = str(a.work / 'bin/kubectl-ate')
results = []
started = datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z')
targets = json.loads((Path(__file__).resolve().parents[2] / 'config/targets.json').read_text())

def command(*args):
    return subprocess.check_output(args, text=True, timeout=45)

def logs(proxy):
    return command('kubectl', '--context', context, '-n', 'vpc-proxy', 'logs', 'deployment/proxy-vpc-pod-' + str(proxy))

@contextlib.contextmanager
def forward(actor):
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    proc = subprocess.Popen([cli, '--context', context, 'proxy', 'actor', actor, f'{port}:80', '-a', 'ate-demo-egress'], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        deadline = time.monotonic() + 90
        sel = selectors.DefaultSelector()
        sel.register(proc.stdout, selectors.EVENT_READ)
        ready = False
        while time.monotonic() < deadline:
            if proc.poll() is not None:
                raise RuntimeError('actor proxy exited')
            for key, _ in sel.select(timeout=min(1, max(0, deadline - time.monotonic()))):
                line = key.fileobj.readline()
                if 'Proxying' in line or 'Forwarding' in line or 'forwarding' in line or 'Listening' in line or 'localhost' in line:
                    ready = True
                    break
            if ready:
                break
        sel.close()
        if not ready:
            raise RuntimeError('actor proxy readiness timeout')
        yield port
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)

def fetch(i):
    actor = i if isinstance(i, str) else f'actor-{i}'
    row = {'actor': actor, 'phase': a.phase, 'level': 'LIVE_KIND'}
    expected = not (a.phase == 'down-both' and i < 3 or a.phase == f'down-{i}' or a.phase == 'extproc-outage' or a.phase == 'uid-mismatch' and i == 1 or a.phase == 'policy-revoke' and i == 3 or a.phase == 'unknown-actor')
    try:
        with forward(actor) as port:
            # HTTP1 создаёт отдельный transport в актуальном demo fetchClient.
            request = urllib.request.Request(f'http://127.0.0.1:{port}/', data=json.dumps({'url': targets['literal' if a.phase == 'literal-ip' else 'fetch'], 'http1': True}).encode(), headers={'Content-Type': 'application/json'})
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            try:
                response = opener.open(request, timeout=45)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                data = json.load(response)
                ok = response.status == 200 and 200 <= data.get('statusCode', 0) < 400 and bool(data.get('body')) and not data.get('error')
                row.update(origin_status=data.get('statusCode'), fetch_ok=ok, error=data.get('error', ''))
                row['status'] = 'PASS' if fetch_matches(data, response.status, expected) else 'FAIL'
    except Exception as error:
        # Ошибка harness не доказывает ожидаемый отказ actor.
        row.update(status='FAIL', harness_error=str(error))
    return row

def isolation():
    image = (a.work / 'probe-image.txt').read_text().strip()
    targets = []
    for i in (1, 2):
        svc = json.loads(command('kubectl', '--context', context, '-n', 'vpc-proxy', 'get', 'svc', f'proxy-vpc-pod-{i}', '-o', 'json'))
        targets.append(svc['spec']['clusterIP'])
        endpoints = json.loads(command('kubectl', '--context', context, '-n', 'vpc-proxy', 'get', 'endpointslices', '-l', f'kubernetes.io/service-name=proxy-vpc-pod-{i}', '-o', 'json'))
        for item in endpoints['items']:
            for ep in item['endpoints']:
                targets.extend(ep['addresses'])
    with forward('actor-1') as port:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        for target in set(targets + ['169.254.169.254', '127.0.0.1', '10.96.0.1']):
            window = datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z')
            request = urllib.request.Request(f'http://127.0.0.1:{port}/connect', data=json.dumps({'proxy': target + ':8443', 'target': '1.1.1.1:443', 'tls': True}).encode(), headers={'Content-Type': 'application/json'})
            try:
                response = opener.open(request, timeout=30)
            except urllib.error.HTTPError as e:
                response = e
            with response:
                data = json.load(response)
            events = command('kubectl', '--context', context, '-n', 'ate-system', 'logs', 'deployment/atenet-egress', '-c', 'ext-proc', '--since-time=' + window)
            gateway_denied = False
            for line in events.splitlines():
                event = json.loads(line)
                if (event.get('msg') == 'vpc_proxy_denied'
                        and event.get('actor', {}).get('name') == 'actor-1'
                        and event.get('actor', {}).get('atespace') == 'ate-demo-egress'
                        and datetime.datetime.fromisoformat(event['time']) >= datetime.datetime.fromisoformat(window)
                        and event.get('reason') == 'destination_not_allowed'):
                    gateway_denied = True
            denied = actor_denied(data, target, gateway_denied)
            results.append({'level': 'LIVE_KIND_ACTOR', 'target': target, 'status': 'PASS' if denied else 'FAIL', 'gateway_denied': gateway_denied, 'output': data})
    workers = json.loads(command('kubectl', '--context', context, '-n', 'ate-demo-egress', 'get', 'pods', '-l', 'ate.dev/worker-pool=egress', '-o', 'json'))['items']
    if not workers:
        raise RuntimeError('no real worker pod found')
    worker = workers[0]['metadata']['name']
    for index, target in enumerate(sorted(set(targets))):
        name = 'vpc-proxy-check-' + str(time.time_ns())
        profile = Path(__file__).resolve().parents[2] / 'config/probe-profile.json'
        run = subprocess.run(['kubectl', '--context', context, '-n', 'ate-demo-egress', 'debug', 'pod/' + worker, '--container', name, '--image', image, '--profile=restricted', '--custom', str(profile), '--attach=true', '--', '/ko-app/probe', '--tcp', '--address', target + ':8443'], capture_output=True, text=True, timeout=45)
        log = command('kubectl', '--context', context, '-n', 'ate-demo-egress', 'logs', worker, '-c', name)
        denied = ('timeout' in log or 'deadline exceeded' in log) and 'tcp_connected' not in log
        results.append({'level': 'LIVE_KIND_WORKER_EPHEMERAL', 'target': target, 'worker': worker, 'status': 'PASS' if denied else 'FAIL', 'output': log})

try:
    if a.phase == 'actor-isolation':
        isolation()
    elif a.phase in ('smoke', 'concurrent'):
        for i in range(1, 4):
            before = [logs(1).count('connect_open'), logs(2).count('connect_open')]
            row = fetch(i)
            after = [logs(1).count('connect_open'), logs(2).count('connect_open')]
            delta = [x-y for x, y in zip(after, before)]
            if i < 3 and (delta[i-1] < 1 or delta[2-i] != 0) or i == 3 and delta != [0, 0]:
                row['status'] = 'FAIL'
            row['proxy_connect_delta'] = delta
            results.append(row)
        with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
            results.extend(pool.map(fetch, (1, 2, 3)))
        events = command('kubectl', '--context', context, '-n', 'ate-system', 'logs', 'deployment/atenet-egress', '-c', 'ext-proc', '--since-time=' + started)
        route_events = []
        for line in events.splitlines():
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get('msg') == 'vpc_proxy_route':
                route_events.append(event)
        for i in range(1, 4):
            identity = json.loads((a.work / 'actors' / f'actor-{i}.json').read_text())['metadata']
            matches = [event for event in route_events
                       if event.get('actor', {}).get('name') == identity['name']
                       and event.get('actor', {}).get('atespace') == identity['atespace']
                       and event.get('actor_uid') == identity['uid']
                       and event.get('mode') == ('direct' if i == 3 else 'proxy')
                       and (i == 3 or event.get('cluster') == f'vpc_proxy_{i}')]
            if not matches:
                results.append({'status': 'FAIL', 'actor': f'actor-{i}', 'reason': 'missing gateway route evidence'})
            elif i < 3:
                destinations = {f"{event['destination_ip']}:{event['destination_port']}" for event in matches}
                connector_matches = []
                for line in logs(i).splitlines():
                    try:
                        event = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if (event.get('msg') == 'connect_open'
                            and event.get('peer_uri') == f'spiffe://vpc-proxy.local/egress/proxy-{i}'
                            and event.get('destination') in destinations):
                        connector_matches.append(event)
                results.append({'status': 'PASS' if connector_matches else 'FAIL',
                                'actor': identity['name'], 'actor_uid': identity['uid'],
                                'cluster': f'vpc_proxy_{i}', 'destinations': sorted(destinations),
                                'proof': 'gateway route and connector peer URI/destination'})
        (a.evidence / f'{a.phase}-gateway.log').write_text(events)
    else:
        results.extend(fetch(i) for i in ([a.negative_actor] if a.phase == 'unknown-actor' else [1, 2] if a.phase == 'literal-ip' else range(1, 4)))
except Exception as error:
    results.append({'status': 'FAIL', 'phase': a.phase, 'harness_error': str(error)})
    raise
finally:
    (a.evidence / f'{a.phase}.json').write_text(json.dumps(results, indent=2))
print(json.dumps(results, indent=2))
raise SystemExit(0 if results and all(r['status'] == 'PASS' for r in results) else 1)
