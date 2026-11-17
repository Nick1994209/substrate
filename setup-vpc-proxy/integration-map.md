# Подключение

| Файл/символ | Изменение | Вызывается | Проверка |
|---|---|---|---|
| router/config.go: routerConfig.validate | opt-in путь, только Envoy egress | startup | router tests |
| router/cmd.go: bindRouterFlags | --vpc-proxy-config | CLI | build |
| router/router.go | строгая загрузка registry | startup | build |
| egress/egress.go | Option, возврат actor из validateActor, hook до policy | CONNECT после authentication/state | egress tests |
| egress/vpc_proxy.go | UID resolve, public target, HeaderMutation | handleConnect | egress tests |

Базовый manifests/ate-install/atenet-egress.yaml не изменяется.

| Файл/символ | Изменение | Вызывается | Проверка |
|---|---|---|---|
| egress/vpc_proxy_test.go | сертификаты и state до routing, UID, revoked, spoof, direct deny | go test | race, count=3 |
| egress/vpc_proxy_envoy_test.go | outer CONNECT с test CA, Lua sanitation, upstream URI reject | scripts/test-envoy.sh, build tag vpcproxy_envoy | LOCAL_INTEGRATION |
| setup-vpc-proxy/pkg/connectproxy | numeric target, TLS, streaming и shutdown | cmd/proxy | TLS, buffered payload, half-close, budget, timeout, cancellation |
| setup-vpc-proxy/cmd/render | полный baseline patch, Secrets, validation Pod | scripts/render.sh | renderer regression и live validate |
| setup-vpc-proxy/cmd/pki | две CA и отдельные leaf roles | scripts/pki.sh | проверка certs при повторном запуске |
| setup-vpc-proxy/cmd/probe | HTTP/TCP и TLS controls | CNI/mTLS/isolation harness | LIVE_KIND |
| scripts/test-repeat.sh, repeat-state.py | сравнение UID, cert fingerprints, count и config hash | verify | repeat.json |
| scripts/test-policy.sh, policy-rewrite.py | revoke/restore с актуальными UID/version | verify | policy-revoke.json |
| tests/e2e/evidence.py | origin error не равен network deny; TLS error не равен isolation | actors.py | test_evidence.py |
| scripts/verification-gate.py | BLOCKED для missing evidence, FAIL для stage errors | verify | test_verification_gate.py |

Вне setup-vpc-proxy изменены только четыре разрешённых router/core файла и три egress-файла, перечисленные выше.
