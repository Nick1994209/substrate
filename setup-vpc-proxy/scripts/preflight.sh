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
exec > >(tee "$ARTIFACTS/preflight.log") 2>&1
trap 'echo "BLOCKED: preflight failed" >&2' ERR
for tool in go docker kubectl kind curl openssl python3 rg; do command -v "$tool"; done
date -u
git rev-parse HEAD
git branch --show-current
git status --short
go version
test "$(go env GOVERSION)" = "$(awk '$1 == "toolchain" {print $2}' go.mod)"
kubectl version --client
"$ROOT/hack/kind.sh" version
df -h "$ROOT"
docker version
docker info --format 'architecture={{.Architecture}} memory={{.MemTotal}}'
