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
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROJECT="$ROOT/setup-vpc-proxy"
WORK="$PROJECT/.work"
ARTIFACTS="$PROJECT/artifacts"
export KIND_CLUSTER_NAME=substrate-vpc-proxy
export KUBECTL_CONTEXT=kind-substrate-vpc-proxy
export KUBECONFIG="$WORK/kubeconfig"
export ATE_INSTALL_KIND=true
export KO_DOCKER_REPO=localhost:5001
case "$(uname -m)" in arm64|aarch64) export KO_DEFAULTPLATFORMS=linux/arm64 ;; x86_64) export KO_DEFAULTPLATFORMS=linux/amd64 ;; *) exit 1 ;; esac
export PATH="$WORK/bin:$PATH"
umask 077
mkdir -p "$WORK/bin" "$ARTIFACTS"
# Стенд использует только public images и собственный локальный registry.
if [[ -z "${VPC_PROXY_DOCKER_CONFIG:-}" ]]; then
  if [[ -z "${DOCKER_HOST:-}" ]]; then
    export DOCKER_HOST
    DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
  fi
  mkdir -p "$WORK/docker"
  printf '{"auths":{}}\n' > "$WORK/docker/config.json"
  export DOCKER_CONFIG="$WORK/docker"
else
  export DOCKER_CONFIG="$VPC_PROXY_DOCKER_CONFIG"
fi
cd "$ROOT"
k() { kubectl --context="$KUBECTL_CONTEXT" --request-timeout=30s "$@"; }
ate() { "$WORK/bin/kubectl-ate" --context="$KUBECTL_CONTEXT" "$@"; }

owned() {
  test -s "$WORK/cluster-id"
  test "$(k get ns kube-system -o jsonpath='{.metadata.uid}')" = "$(cat "$WORK/cluster-id")"
}

ensure_proxy_namespace() {
  if k get namespace vpc-proxy >/dev/null 2>&1; then
    test -s "$WORK/proxy-namespace-id"
    test "$(k get namespace vpc-proxy -o jsonpath='{.metadata.uid}')" = "$(cat "$WORK/proxy-namespace-id")"
    test "$(k get namespace vpc-proxy -o jsonpath='{.metadata.labels.app\.kubernetes\.io/part-of}')" = setup-vpc-proxy
  else
    k create namespace vpc-proxy
    k label namespace vpc-proxy app.kubernetes.io/part-of=setup-vpc-proxy
    k get namespace vpc-proxy -o jsonpath='{.metadata.uid}' > "$WORK/proxy-namespace-id"
  fi
}
