# PROMPT 03 — registry и реальный Go hook в atenet-egress

Не ограничивайся созданием нового пакета. Реализуй весь путь flag → startup loader → handler option → authenticated CONNECT → Envoy HeaderMutation.

## Пакет bindings

Создай setup-vpc-proxy/pkg/bindings. Только независимые stdlib/YAML типы; **не импортировать** cmd/atenet/internal/router/extproc и internal/egresspolicy. Сигнатуры и config — в reference/code-contract.go.txt и reference/routes.example.yaml.
Используй строгий YAML/JSON decoder, запрет unknown fields, duplicate keys/actors/UID/cluster mismatch, limit размера файла. Не проверять actor existence через Kubernetes: actors живут в ate API.
Ключи Resolve: atespace + name + текущий server UID. UID из сертификата не угадывать.
Конфиг загружается на startup и immutable. Неверный config останавливает gateway. Нет неявного default direct и нет hot reload без реализации.

Прокси-кластеры разрешены только `vpc_proxy_1` и `vpc_proxy_2` и имеют жёсткое соответствие своим Service/cert binding. `direct` и `revoked` не могут содержать cluster.
`Decision.Mode`: proxy/direct/revoked. Любая непредусмотренная mode запрещает доступ. Ошибки Missing/UIDMismatch не должны терять различимость для server logs, но actor получает generic denial.
Запретить две записи с одинаковым atespace/name даже при разных UID. Для нового UID требуется явная перегенерация config.

## Узкие core-изменения — обязательны

### config.go

В routerConfig добавить `VPCProxyConfigFile string`.
В startup validation запретить flag при ingress mode или agentgateway dataplane: MVP поддерживает только Envoy egress. Пустой flag сохраняет baseline.

### cmd.go

В bindRouterFlags добавить:

```go
fs.StringVar(&cfg.VPCProxyConfigFile, "vpc-proxy-config", "",
    "Opt-in actor-aware egress registry YAML; empty preserves legacy behavior")
```

### router.go

В блоке `if s.cfg.Mode.ServesEgress()` загрузить bindings.LoadFile ДО регистрации handler:

```go
var opts []egress.Option
if s.cfg.VPCProxyConfigFile != "" {
    reg, err := bindings.LoadFile(s.cfg.VPCProxyConfigFile)
    if err != nil {
        return fmt.Errorf("loading --vpc-proxy-config: %w", err)
    }
    opts = append(opts, egress.WithVPCProxyRegistry(reg))
}
eg := egress.New(s.apiClient, loadActorIdentityRoots,
    s.cfg.EgressPolicyCacheTTL, providers, opts...)
handlers[eg.Direction()] = eg
```

Добавить импорт `go.ate.dev/substrate/setup-vpc-proxy/pkg/bindings`.
Нельзя обращаться отсюда к файлу в pod, если mount и flag не добавлены в deploy overlay.

### egress.go / небольшой vpc_proxy.go

Сделать backward-compatible variadic constructor: старые вызовы New(4 arguments) должны компилироваться.

```go
type Option func(*Handler)
func WithVPCProxyRegistry(reg *bindings.Registry) Option {
    return func(h *Handler) { h.vpcProxyRegistry = reg }
}
// New(...existing four parameters..., opts ...Option) *Handler
// После создания h применить opts, вернуть h.
```

Добавить поле `vpcProxyRegistry *bindings.Registry`. nil означает только feature off, не «ошибка загрузки».
Изменить validateActor на `(*ateapi.Actor, error)` с сохранением ВСЕХ текущих state/API error checks. Во всех return error добавить nil actor; в RUNNING/RESUMING вернуть actor, nil. Обновить вызовы/тесты. Не делать второй GetActor ради UID.

В handleConnect сохранить порядок:
1. проверка CONNECT;
2. загрузка actor identity CA;
3. штатная authenticateActorCertificate;
4. штатная validateActor, возвращающая actor;
5. существующая NormalizeAuthority, numeric IP:port;
6. **НОВАЯ ветка** registry, если включена;
7. исходный lookupPolicy/SNIRules/connectMetadata/allow для explicit direct или feature off.

Новая ветка использует `reg.Resolve(actorRef.Atespace, actorRef.Name, actor.GetMetadata().GetUid())`.
Resolve error → 403/503 по зафиксированной схеме, никогда legacy. Для proxy вызвать общий `netpolicy.CheckPublicTarget(dest.IP, uint16(dest.Port))`; В baseline Port уже uint16: достаточно проверить != 0; если другой checkout использует int/string, проверить 1..65535 до преобразования, без truncation.
revoked → 403. direct → не возвращать allow преждевременно, продолжить baseline.

Proxy decision преобразовать в extproc Result **внутри разрешённого cmd/atenet/internal/router/egress package**. Образец точной mutation — reference/egress-hook.go.txt.
Это важно: Go internal import rules запрещают helper под setup-vpc-proxy импортировать cmd/atenet/internal/... . Не обходи это через копирование extproc types.

## Metadata и логи

Сохрани существующие set_filter_state и identity propagation. `connectMetadata(dest,nil)` допустимо сохранить для совместимости metadata readers на outer leg, но оно не должно включать внутренний MITM route для proxy.
`res.Target` — диагностическое поле, **не меняет маршрут само**: требуется HeaderMutation + Envoy route.
Нужны структурированные события gateway: `vpc_proxy_route`, actor atespace/name/UID, mode, cluster, numeric destination, результат. Для direct тоже событие, без proxy cluster. Не логировать сертификаты, ключи, токены и payload.

## Unit gate

- Старые tests egress/route/auth всё ещё проходят.
- Registry feature off не меняет baseline.
- actor-1 и actor-2 с тем же destination получают разные cluster mutation.
- explicit direct не получает routing header и сохраняет policy denial.
- Unknown actor / UID mismatch / revoked не дают allow.
- spoofed header не участвует в Resolve.
- authenticated actor state и certificate checks не обходятся proxy route.
- Поддельный SPIFFE header без валидного сертификата не даёт доступ.

Готовность: `go test` новых пакетов и изменённых core packages + настоящий `go build ./cmd/atenet`. Зафиксировать команды и результаты.

## Обязательная точность ограничений

Если actor GetActor недоступен, сохранить исходную fail-closed обработку API ошибки. Registry lookup — memory-only; не делать сетевых запросов из Resolve.
При compatibility port сохранить UID/purpose verification из сертификата, если она уже реализована upstream. Нельзя откатывать её к name-only identity из старого SHA.
Не размножать Option/WithVPCProxyRegistry при копировании образца: определения должны быть ровно в одном файле пакета egress.
