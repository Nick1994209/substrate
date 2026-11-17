# Security review

Рассмотрены фактический Go path и scripts. Стенд не объявляется production ready.
Независимый agent review выполнен по AGENTS.md. Подтверждённые замечания исправлены.

| Severity | Путь | Сценарий | Исправление | Evidence |
|---|---|---|---|---|
| Medium | connectproxy/server.go | idle TLS/HTTP исчерпывают ресурсы до CONNECT | capped listener и HTTP idle timeout | TestListenerBudget, TestRunCancellation, race |
| Medium | render/envoy.go | virtual host выключает ext_proc | отказ при overrides и неизвестном pipeline | TestPatchEnvoy |
| Medium | bindings/registry.go | один UUID записан в разных регистрах | только canonical lowercase UID | строгая валидация registry |
| Medium | scripts/validate.sh | первый deploy делает dry-run в отсутствующем namespace | namespace создаётся с UID/label-маркером до validation | live validation PASS |
| Medium | scripts/cleanup.sh | удаление чужого namespace | проверка UID и label | review scripts |
| Medium | install-substrate.sh | неуспешный smoke принимается за готовый baseline при retry | отдельный baseline-ready marker | review scripts |
| Medium | bootstrap-kind.sh | reset повторно использует UID/state старого кластера | архив cluster-bound state | review scripts |
| Medium | create-actors.sh | CLI error оставляет пустой actor marker | временный файл и atomic rename | review scripts |

Identity читается только из проверенного actor certificate. Proxy route выполняется после state check.
Неверные registry, UID и unknown actors запрещены. Direct проходит штатную policy.
Ключи gateway не монтируются в ext-proc или connectors. CA private keys остаются локально.
Публичные адреса и порты проверяются и gateway, и connector.

Выявлено и устранено ограничение доступности: первая gateway Egress policy с одним connector allow отрезала бы DNS/API. Отдельная gateway policy разрешает точные системные selectors и public TCP 80/443 с теми же deny CIDR, что connectors. TestGatewayNetworkDependencies проверяет её состав и idempotence.

Остаточные ограничения перечислены в [docs/limitations.md](docs/limitations.md).
Дополнительный streaming test выявил обрезание ответа при half-close. После hijack request context больше не завершает tunnel; используется shutdown channel сервера. TestHalfClosePreservesResponse проходит с race detector.

Live actor/worker isolation, CNI allow/deny/allow, mTLS exact URI и раздельные proxy outages прошли.
Итоговая matrix и ограничения общего repository suite находятся в [docs/verification.md](docs/verification.md).
После runtime review исправлены forwarding sessions, nonroot probe UID, ожидание удаления старых gateway Pods и обработка частичных результатов harness.
