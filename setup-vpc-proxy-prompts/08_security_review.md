# PROMPT 08 — независимый security review

Переключись в режим security reviewer. Оцени фактический diff и runtime config, а не намерения предыдущих сообщений. Исправь critical/high issues, повтори соответствующие тесты. Не размывай инварианты ради зелёных checks.

## Проверить code path

- Flag зарегистрирован, путь смонтирован, loader вызывается, Option передана, handler использует registry.
- Authentication actor и state check выполняются ДО binding resolution.
- Name/atespace не берутся из actor-controlled header, URL или body.
- TLS1.3 + CA chain + EKU/time + exact URI у proxy; Envoy проверяет server CA + exact server URI.
- Gateway действительно имеет ДВА разных client key/cert и два cluster transports.
- Нет fall-open при malformed config, отсутствующем actor/UID, cluster error, ext-proc timeout, failed TLS.
- Header sanitation перед ext_proc, routes фиксированные, unknown header denied. ext_proc не выключен per-route. ClearRouteCache применяется и не вводит обход auth.
- Proxy получает numeric original authority, не Service address и не произвольный hostname.
- Ограничения public destinations и ports одинаковы на gateway/proxy, включая mapped IPv6 и специальные диапазоны. Выбранная IPv4-only модель явно указана.
- Streaming сохраняет buffered bytes, нет неограниченных goroutines/tunnels и забытых hijacked conns.

## Проверить manifests/runtime

- Только dedicated connectors выходят к Internet по заданному allow; actors не получают direct interface/ENV proxy обход.
- Kubernetes NetworkPolicies реально enforced и нет более широкой additive allow.
- Service selectors не включают оба proxies, endpoints соответствуют одному binding.
- Read-only ФС, non-root, caps drop, no host creds/hostPath/docker.sock, токены SA отключены.
- Удалённые/устаревшие UID не разрешаются автоматически.
- Изменения за пределами setup-vpc-proxy минимальны, не скопировано полпроекта.
- Secrets не в git/logs/artifacts. Pinned image digests совпадают с реально запущенными.
- Любые временные test permissions удаляются даже после неуспеха теста.

## Не приписывать отсутствующие гарантии

- gVisor — user-space kernel boundary, не hardware microVM.
- Current SVID name reuse/freshness limitation не решена одним GetActor UID.
- Proxy mode intentionally bypasses HTTP per-request enforcement/MITM/DLP; DNS rules лишь фильтруют DNS.
- Открытые tunnels не отзываются автоматическим изменением static config. Укажи процедуру: revoke config + gateway rollout и/или restart конкретного connector для закрытия соединений. Gateway rollout прекращает также соединения других actors; это допустимое ограничение MVP.
- Разрешённый публичный endpoint может сам быть relay/proxy: public-IP ACL не контролирует последующие действия удалённого сервера.
- DNS идёт существующим relay, не через connector, это исключение из формулировки «весь TCP».
- Нет VPC, private DNS, UDP/QUIC, production CA/SDS rotation или динамического provisioning.

Создай SECURITY.md с findings: severity, affected path, exploit scenario, fix, test evidence, остаточное ограничение. Не писать «production ready». При unresolved critical/high, влияющем на заявленную actor isolation, итог verify должен FAIL.
