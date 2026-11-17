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
k -n ate-system scale deployment/atenet-egress --replicas=0
k -n ate-system wait pod -l app=atenet-egress --for=delete --timeout=180s
for id in 1 2; do
  k -n vpc-proxy scale deployment/"proxy-vpc-pod-$id" --replicas=0
  k -n vpc-proxy wait pod -l "app=actor-vpc-proxy,proxy-id=$id" --for=delete --timeout=180s
done
backup="$WORK/archives/pki-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$WORK/archives"
mv "$WORK/pki" "$backup"
go run ./setup-vpc-proxy/cmd/pki --out "$WORK/pki"
bash "$PROJECT/scripts/deploy.sh"
