# Источники и границы проверки

Подготовлено 2026-11-05. Для исходного wiring просмотрен checkout SHA
`ed0371a15f6f265c64100001adafb9731be03970`.
Ни приложение из будущей реализации, ни kind deployment этим prompt pack не объявляются протестированными.
Примеры reference — новое предлагаемое расширение, не фрагменты уже существующего upstream API.

## Проверенные файлы baseline

Все пути ниже относятся к https://github.com/agent-substrate/substrate/tree/ed0371a15f6f265c64100001adafb9731be03970

| Файл | Что агент должен проверить / использовать |
|---|---|
| `go.mod` | Корневой module path, Go/toolchain, vendoring conventions |
| `README.md` | Установка kind, ate-setup, credentials opt-out |
| `hack/create-kind-cluster.sh` | Feature gates, registry, destructive recreation, сетевые prerequisites |
| `hack/kind.sh` | Pinned kind tooling |
| `cmd/atenet/internal/router/config.go` | routerConfig и validation |
| `cmd/atenet/internal/router/cmd.go` | bindRouterFlags |
| `cmd/atenet/internal/router/router.go` | Регистрация egress handler |
| `cmd/atenet/internal/router/egress/egress.go` | handleConnect, validateActor, authenticateActorCertificate, connectMetadata, allow |
| `cmd/atenet/internal/router/extproc/handler.go` | extproc.Result: Target не переключает upstream |
| `internal/egresspolicy/egresspolicy.go` | Destination.IP netip.Addr, Destination.Port uint16, Destination.Hostname |
| `internal/resources/spiffe.go` | Формат URI SAN именно выбранной ревизии |
| `manifests/ate-install/atenet-egress.yaml` | Outer CONNECT HCM, ext_proc, MITM routes, Secrets, Deployment, NetworkPolicy |
| `demos/egress/README.md` | HTTP fetch API, CLI/policy walkthrough; комментарии могут опережать реализацию |
| `demos/egress/egress-template.yaml.tmpl` | ActorTemplate — ate API resource, image/root CA bundle |
| `demos/egress/egress.yaml.tmpl` | WorkerPool — Kubernetes CRD, baseline replicas 2 |
| `hack/verify-egress-demo.sh` | Воспроизводимые примеры network probes |
| `api/ateapi/v1/ateapi.proto` | Actor UID, EgressPolicy field casing и CLI resource schema |

Не смешивать snapshot pinned source с меняющимся `main`. На момент просмотра main-документация network-egress уже описывает сертификат ActorIdentity с UID/purpose; выбранный baseline handler удостоверяет worker-for-actor URI и возвращает ActorRef без UID. Поэтому в задании явно зафиксировано ограничение старого baseline и требование НЕ ослаблять более новую аутентификацию при порте.

## Официальная документация

- Репозиторий и quickstart: https://github.com/agent-substrate/substrate
- Меняющийся сетевой контракт: https://github.com/agent-substrate/substrate/blob/main/docs/guides/network-egress.md
- Меняющаяся таблица протоколов: https://github.com/agent-substrate/substrate/blob/main/docs/guides/egress-traffic.md
- Envoy CONNECT / upgrades: https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/upgrades.html
- Envoy route components: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/route/v3/route_components.proto
- Envoy ext_proc: https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_proc/v3/ext_proc.proto
- Envoy HeaderMutationRules: https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/common/mutation_rules/v3/mutation_rules.proto
- Kubernetes NetworkPolicy: https://kubernetes.io/docs/concepts/services-networking/network-policies/
- kind configuration: https://kind.sigs.k8s.io/docs/user/configuration/
- Go crypto/tls: https://pkg.go.dev/crypto/tls
- Go net/http Hijacker и Shutdown: https://pkg.go.dev/net/http

Полный сгенерированный Envoy config валидировать тем же image/version, который запущен в стенде, а не просто по latest docs.
