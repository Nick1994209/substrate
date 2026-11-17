#!/usr/bin/env bash

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

set -Eeuo pipefail
source "$(dirname "$0")/common.sh"

bash "$PROJECT/scripts/install-kubectl.sh"

# Кластер с таким именем без локального маркера не трогаем.
if kind get clusters | rg -qx "$KIND_CLUSTER_NAME"; then
  owned
  existing=1
  if [[ "${RESET_CLUSTER:-0}" == 1 ]]; then
    kind delete cluster --name "$KIND_CLUSTER_NAME"
    existing=0
    backup="$WORK/archives/$(date -u +%Y%m%dT%H%M%SZ)"
    mkdir -p "$backup"
    for entry in "$WORK"/*; do
      case "$(basename "$entry")" in bin|archives) continue ;; esac
      mv "$entry" "$backup/"
    done
  elif [[ -f "$WORK/bootstrap-ready" ]]; then
    k wait node --all --for=condition=Ready --timeout=180s
    exit 0
  fi
fi
registry=substrate-vpc-proxy-registry
if docker container inspect "$registry" >/dev/null 2>&1; then
  test "$(docker inspect -f '{{index .Config.Labels "setup-vpc-proxy"}}' "$registry")" = owned
  test "$(docker inspect -f '{{.State.Running}}' "$registry")" = true
  docker port "$registry" 5000/tcp | rg -qx '127.0.0.1:5001'
else
  docker run -d --restart=unless-stopped --label setup-vpc-proxy=owned \
    -p 127.0.0.1:5001:5000 --name "$registry" \
    registry:3@sha256:38907510915c3f1604c7e83d6ab602ab3020d092f73bdaed8696773693ff768c
fi
cp "$PROJECT/config/kind.yaml" "$WORK/kind.yaml"
if [[ "${existing:-0}" != 1 ]]; then
  "$ROOT/hack/kind.sh" create cluster --name "$KIND_CLUSTER_NAME" --config "$WORK/kind.yaml" --kubeconfig "$KUBECONFIG" --wait 0s
fi
k get ns kube-system -o jsonpath='{.metadata.uid}' > "$WORK/cluster-id"
k label ns kube-system setup-vpc-proxy=owned
for node in $(kind get nodes --name "$KIND_CLUSTER_NAME"); do
  docker exec "$node" sysctl net.ipv4.conf.all.proxy_arp=1
  docker exec "$node" sysctl -e net.ipv6.conf.all.proxy_ndp=1
  docker exec "$node" mkdir -p /etc/containerd/certs.d/localhost:5001
  docker exec -i "$node" cp /dev/stdin /etc/containerd/certs.d/localhost:5001/hosts.toml < "$PROJECT/config/hosts.toml"
done
if [[ "$(docker inspect -f '{{json .NetworkSettings.Networks.kind}}' "$registry")" == null ]]; then
  docker network connect kind "$registry"
fi
k apply -f "$PROJECT/manifests/local-registry.yaml"
bash "$PROJECT/scripts/install-cni.sh"
k wait node --all --for=condition=Ready --timeout=300s
bash "$PROJECT/scripts/test-network-policy.sh"

touch "$WORK/bootstrap-ready"
