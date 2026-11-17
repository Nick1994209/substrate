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

if [[ -x "$WORK/bin/kubectl" ]]; then
  "$WORK/bin/kubectl" version --client -o json | python3 -c 'import json,sys; assert json.load(sys.stdin)["clientVersion"]["gitVersion"] == "v1.37.0"'
  exit 0
fi
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64) arch=amd64 ;; *) exit 1 ;; esac
url="https://dl.k8s.io/release/v1.37.0/bin/$os/$arch/kubectl"
curl --fail --location --max-time 30 "$url.sha256" -o "$WORK/bin/kubectl.sha256"
if ! python3 "$PROJECT/scripts/checksum.py" "$WORK/bin/kubectl.tmp" "$WORK/bin/kubectl.sha256"; then
  curl --fail --location --continue-at - --retry 3 --retry-all-errors --max-time 600 "$url" -o "$WORK/bin/kubectl.tmp"
fi
python3 "$PROJECT/scripts/checksum.py" "$WORK/bin/kubectl.tmp" "$WORK/bin/kubectl.sha256"
chmod 0755 "$WORK/bin/kubectl.tmp"
mv "$WORK/bin/kubectl.tmp" "$WORK/bin/kubectl"
