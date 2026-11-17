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

owned
curl --fail --location --max-time 60 https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml -o "$WORK/calico.original.yaml"
# Образы заменяются по проверенному digest lock, без latest.
python3 "$PROJECT/scripts/prepare-cni.py" "$WORK/calico.original.yaml" "$PROJECT/config/cni-images.json" "$WORK/calico.yaml"
k apply --server-side -f "$WORK/calico.yaml"
k -n kube-system rollout status daemonset/calico-node --timeout=300s
k -n kube-system rollout status deployment/calico-kube-controllers --timeout=300s
