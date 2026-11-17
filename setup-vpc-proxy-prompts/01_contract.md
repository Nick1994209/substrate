# PROMPT 01 — единый контракт и файловая структура

Реализуй ровно три маршрута: proxy-1, proxy-2 и explicit direct. Не разрабатывай полноценный controller/xDS/VPN продукт.
Сохраняй штатную actor-аутентификацию, actor state check и исходный direct pipeline.

## Обязательное файловое дерево реализации

```text
setup-vpc-proxy/
  README.md BASELINE.md PROGRESS.md integration-map.md SECURITY.md
  Makefile .gitignore
  cmd/proxy/main.go
  cmd/render/main.go
  cmd/pki/main.go                 # предпочтительно crypto/x509, допустим shell openssl
  pkg/bindings/config.go
  pkg/bindings/registry.go
  pkg/bindings/registry_test.go
  pkg/netpolicy/public.go
  pkg/netpolicy/public_test.go
  pkg/connectproxy/server.go
  pkg/connectproxy/tls.go
  pkg/connectproxy/server_test.go
  pkg/connectproxy/tls_test.go
  config/routes.example.yaml
  config/targets.yaml
  manifests/proxy.yaml.tmpl
  manifests/networkpolicies.yaml.tmpl
  manifests/actor-policy.yaml.tmpl
  scripts/common.sh
  scripts/preflight.sh
  scripts/bootstrap-kind.sh
  scripts/install-substrate.sh
  scripts/build-images.sh
  scripts/create-actors.sh
  scripts/render.sh
  scripts/deploy.sh
  scripts/smoke.sh
  scripts/test-mtls.sh
  scripts/test-fail-closed.sh
  scripts/test-network-policy.sh
  scripts/verify.sh
  scripts/collect-evidence.sh
  scripts/cleanup.sh
  tests/e2e/                    # harness и fixtures
  docs/architecture.mmd
  docs/architecture.md
  docs/verification.md
  docs/limitations.md
  .work/                       # gitignored: state, PKI, rendered YAML, backups
  artifacts/                   # gitignored: logs, junit/json, evidence
```

Дополнительные файлы разрешены, но не вместо перечисленных функций. Сводка функций — в reference/code-contract.go.txt.
Весь новый код должен импортироваться или вызываться из реально исполняемого пути. Нельзя отвечать «создал registry» без CLI flag + bootstrap wiring + mount.

## Режимы

- Feature flag `--vpc-proxy-config` отсутствует: исходное поведение проекта полностью сохраняется, regression tests проходят.
- Flag указан: конфиг должен существовать и полностью валидироваться на startup. Ошибка → startup failure. Нельзя игнорировать ошибки и уходить в legacy.
- При включённом feature только три явных actor records: actor-1/proxy-1, actor-2/proxy-2, actor-3/direct.
- Неизвестная identity, отсутствующий UID или UID mismatch → deny.
- `proxy` → только заранее заданный cluster; нельзя direct fallback даже при неработающем proxy.
- `direct` → исходный lookupPolicy и все последующие проверки; не безусловный allow.
- `revoked` → deny. Для MVP поддерживается как минимум новый config + рестарт gateway.

Конфиг статический, загружается один раз при startup. ConfigMap changes без rollout не обещать. Не добавлять фальшивую TTL/ready динамику: при падении proxy сеть вернёт ошибку, а не сменит маршрут.

## Scope security

Для proxy actors разрешены лишь публичные IP и TCP 80/443. Это явно включаемый public CONNECT mode, **обходящий штатный HTTP MITM/DLP/credential injection**. DNS разрешения в EgressPolicy не превращаются в HTTP hostname ACL: по числовому публичному IP actor может достичь любого разрешённого публичного endpoint. Этот trade-off MVP явно описать.
Не объявляй это безопасным production multi-tenant VPC сервисом. В частности, текущий worker-for-actor SVID содержит имя/atespace, но не UID: UID binding предотвращает старый registry после пересоздания actor, но сам по себе не защищает новый actor от старого ещё действующего SVID с тем же именем. В MVP запрети переиспользование имён в пределах стенда; production требует identity generation/freshness extension.

## Преднамеренно исключено

Нет private DNS/VPC routing, VPN, UDP, QUIC, ICMP, Kubernetes operator, CRD для binding, автоматического выпуска SPIFFE identity через SPIRE, динамических SDS/CDS/EDS, миграции открытых TCP при snapshot. Не менять actor protocol для прокси и не класть HTTP_PROXY в actors.

Готовность этого этапа: дерево создано, security decisions зафиксированы, точные внешние glue-изменения согласованы с реальным исходником.
