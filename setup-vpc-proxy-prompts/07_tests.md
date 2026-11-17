# PROMPT 07 — обязательная проверка маршрутов и изоляции

Реализуй tests как исполняемые команды с ненулевым exit при провале. Итог — machine-readable JSON/JUnit и human-readable matrix. SKIP/BLOCKED не являются PASS.

## Unit / локальная integration

- bindings config strict parse, duplicates/unknown fields, missing actor, real UID mismatch, direct/proxy/revoked invariants;
- policy special/public addresses, private/RFC1918/metadata/CGNAT/IPv4-mapped IPv6/reserved/port checks;
- Go proxy mTLS positive и negative, buffered payload, двусторонний stream, dial error, deadlines, resource limits, shutdown;
- gateway feature flag default off regression, true actor certificate auth, lookup до policy для proxy, direct policy still enforced;
- spoofed routing/identity headers не выбирают binding;
- renderer idempotent, unexpected baseline shape fails, only outer listener modified, unknown header fails deny;
- package `cmd/atenet/internal/...` imports не протекли в setup-vpc-proxy packages;
- `go build ./cmd/atenet`, build proxy, targeted `go test` и при доступности race для новых пакетов.

Тесты TLS проверяют TLS, не используют insecure режимы. Wrong-URI под тем же trusted CA обязателен: cross-CA failure сам по себе не проверяет exact URI SAN logic.

## Runtime checks

### 1. Реальные actors

Создай 3 actor resources в Substrate, проверь Running/Resuming и gVisor runtime; не подменять kubectl run probes.
Сделай POST в egress demo actor через `kubectl ate proxy actor` и JSON url. Успешный HTTP status внешнего handler ещё не доказывает success fetch: разобрать JSON, проверить origin statusCode, непустое тело, отсутствие error. URL задавать через fixture config.
У demo http client возможно connection reuse: для path/failure tests нужны **новые CONNECT**, а не заранее открытые tunnels. Реализуй controller harness с fresh actor client fixture либо явно отключай origin keep-alives в новом actor-probe под setup-vpc-proxy. Не полагайся на random query string для закрытия TCP: она не гарантирует новое соединение. Допустимо сначала scale down proxy, дождаться исчезновения endpoints/close streams, затем новый запрос. Для gateway/client-state tests предусмотрен restart actor fixture или отдельная fresh-connection команда.

### 2. Доказательство трёх путей

Одинаковый публичный destination у actor-1 и actor-2 должен давать:
- gateway event actor-1, UID1, cluster=vpc_proxy_1;
- proxy-1 connect_open, expected client URI1, тот же literal destination;
- отсутствие соответствующего события в proxy-2;
- симметрично для actor-2;
- actor-3 gateway mode=direct + baseline pipeline, **нет** новых соответствующих CONNECT в обоих proxies.

Корреляция: timestamps/test window + destination/port + per-actor gateway event + per-proxy counters, последовательные и concurrent этапы. В proxy logs нет автоматически actor UID, т.к. его identity — per-binding client SAN; сопоставлять через проверенный binding map. Если добавляешь correlation header, выставляет его только gateway и он не становится источником identity.
Не использовать разные внешние IP как единственное доказательство: в kind у всех путей может быть одинаковый NAT IP.

### 3. Раздельные отказы

| Действие | actor-1 | actor-2 | actor-3 |
|---|---|---|---|
| Оба proxy Ready | fetch OK через proxy-1 | fetch OK через proxy-2 | direct OK |
| proxy-1 replicas=0 | FAIL, без direct/proxy-2 fallback | OK | OK |
| proxy-1 восстановлен | OK | OK | OK |
| proxy-2 replicas=0 | OK | FAIL, без direct/proxy-1 fallback | OK |
| Оба proxy replicas=0 | FAIL | FAIL | direct OK |
| ext-proc не отвечает | fail closed | fail closed | fail closed |
| Отозвана policy actor-3 | не менять | не менять | direct denied |

Перед каждым падением ждать Deployment/endpoints state, после каждого restore ждать readiness. Trap обязан восстановить состояние даже при test fail. Для actor-3 policy revoke тестировать актуальный policy cache TTL и новый CONNECT/request; bounded wait, не sleep наугад. Имеющаяся DNS cache не должна превращать тест в ложный успех.

### 4. mTLS

Внутри отдельного доверенного test namespace, не actor:
- client1 + trust server1 → proxy1 TLS/CONNECT OK;
- client2 → proxy1 DENY;
- client1 → proxy2 DENY;
- без client cert → DENY;
- чужая CA → DENY;
- trusted CA + wrong client URI → DENY;
- expired client → DENY;
- сервер с trusted CA, но wrong server URI → Envoy upstream DENY;
- plaintext на 8443 не проксирует ничего.

Negative certificates только краткоживущие test fixtures, production CA private keys остаются на workstation, после теста secrets/Pod удалить.
Для gateway-side server SAN test локальный Envoy integration fixture допустим, но помечай LOCAL_INTEGRATION, не выдавай его за live runtime mutation, если не выполнял.
Service ClusterIP нельзя проверять из host curl без маршрута: использовать in-cluster test client или отдельный port-forward. Нельзя открывать всем ingress ради теста. Temporary narrowly scoped NetworkPolicy allow только для trusted test Pod, убрать trap-ом.

### 5. Network/isolation

- CNI enforcement: allow-before-deny/deny/allow positive control.
- Из actor-1 попытка TCP к Service/Pod IP обоих connectors → denied, не попадёт на mTLS listener напрямую.
- Private/linklocal/node/service/metadata CONNECT destinations → denied.
- Literal public IP/443 работает у proxy actor даже без DNS lookup — это ожидаемый задокументированный public TCP scope, не ошибка политики.
- Actor без явной routes record denied; этот fourth actor только краткоживущий negative fixture, без третьего connector. После теста удалить.
- actor UID mismatch в registry → deny и отсутствие прямого fallback.
- new actor с тем же именем не разрешать автоматически. Ограничение старого SVID описать, не утверждать, что одним UID lookup оно устранено.
- Request с x-ate-vpc-cluster=vpc_proxy_2 от actor-1 не должен изменить маршрут. Проверять где возможно на аутентифицированном outer CONNECT client fixture, а не лишь внутри HTTP payload: это разные уровни протокола.
- Не давать test actors приватные ключи atunnel; для authenticated outer-protocol теста использовать изолированный test fixture control plane/сертификатов или host-owned privileged harness и явно обозначить уровень теста.

## Evidence

Сохранять реальные версии/команды/exit codes, pod names, image digests, server/client cert **публичные** fingerprints и URI SAN, registry без ключей, sanitized route excerpt, структурированные logs/counters, результаты fail-closed matrix и duration. Не сохранять Secret.data/private keys/full pod ENV в evidence.
Собери `make verify` так, чтобы failed stage не затерялся в pipe/tee. В отчёте разделять UNIT, LOCAL_INTEGRATION, LIVE_KIND и BLOCKED.

## Нельзя подделывать actor-controlled outer header

В обычном actor sandbox внешний CONNECT делает доверенный atunnel. Заголовок x-ate-vpc-cluster внутри actor HTTP payload не является заголовком outer CONNECT. Поэтому sanitation для внешнего CONNECT доказывай отдельным локальным Envoy+ext_proc integration test с валидным test actor certificate и test CA; не копируй runtime atunnel keys из worker в actor. В live-тесте actor-controlled HTTP header допустим как дополнительный, но не заменяющий тест.
