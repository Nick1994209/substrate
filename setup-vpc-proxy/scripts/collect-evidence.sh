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
k version -o json > "$ARTIFACTS/kubernetes.json"
k -n ate-system get deployment atenet-egress -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{" "}{.image}{"\n"}{end}' > "$ARTIFACTS/gateway-images.txt"
k -n ate-system get pods -l app=atenet-egress -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .status.containerStatuses[*]}{.name}{" "}{.imageID}{"\n"}{end}{end}' > "$ARTIFACTS/gateway-imageids.txt"
k -n vpc-proxy get pods -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .status.containerStatuses[*]}{.imageID}{"\n"}{end}{end}' > "$ARTIFACTS/proxy-images.txt"
k -n ate-system logs deployment/atenet-egress -c ext-proc --since=10m | rg 'vpc_proxy_' > "$ARTIFACTS/gateway-routes.log"
for id in 1 2; do
  k -n vpc-proxy logs deployment/"proxy-vpc-pod-$id" --since=10m > "$ARTIFACTS/proxy-$id.log"
  openssl x509 -in "$WORK/pki/$id/client.crt" -noout -fingerprint -sha256 -ext subjectAltName > "$ARTIFACTS/client-$id.txt"
  openssl x509 -in "$WORK/pki/$id/server.crt" -noout -fingerprint -sha256 -ext subjectAltName > "$ARTIFACTS/server-$id.txt"
done
cp "$WORK/routes.yaml" "$WORK/images.json" "$WORK/rendered/config.sha256" "$ARTIFACTS/"
