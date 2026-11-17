# Проверки

| Уровень | Проверка | Статус | Evidence |
|---|---|---|---|
| UNIT | strict registry, public targets, proxy, renderer, router regression | PASS | [unit.jsonl](../artifacts/unit.jsonl), count=3/shuffle/race |
| LOCAL_INTEGRATION | proxy mTLS, buffered stream, half-close в обоих направлениях, limits/shutdown | PASS | [unit.jsonl](../artifacts/unit.jsonl) |
| LOCAL_INTEGRATION | custom Envoy outer sanitation, trusted CA wrong server URI | PASS | [envoy-local.log](../artifacts/envoy-local.log), count=3/shuffle |
| BUILD | gateway и connector, immutable images | PASS | [images.json](../artifacts/images.json) после collect-evidence |
| LIVE_KIND | Substrate, gVisor workers, golden snapshot, baseline fetch | PASS | [bootstrap-resume.log](../artifacts/bootstrap-resume.log), [baseline.json](../artifacts/baseline.json) |
| LIVE_KIND | CNI allow/deny/allow | PASS | [cni.json](../artifacts/cni.json) |
| LIVE_KIND | actual Envoy config validation и server dry-run | PASS | [envoy-validation.log](../artifacts/envoy-validation.log) |
| LIVE_KIND | actors 1→proxy1, 2→proxy2, 3→direct, concurrent | PASS | [smoke.json](../artifacts/smoke.json), [smoke-gateway.log](../artifacts/smoke-gateway.log) |
| LIVE_KIND | literal IPv4 fetch | PASS | [literal-ip.json](../artifacts/literal-ip.json) |
| LIVE_KIND_PORT_FORWARD | positive/cross/no cert/wrong URI/expired/EKU/foreign CA, server URI | PASS | [mtls.json](../artifacts/mtls.json), 28 случаев |
| LIVE_KIND | actor Service/Pod/private IP и real worker ephemeral TCP isolation | PASS | [actor-isolation.json](../artifacts/actor-isolation.json) |
| LIVE_KIND | proxy1=0, proxy2=0, оба=0, восстановление каждого | PASS | [down-1.json](../artifacts/down-1.json), [down-2.json](../artifacts/down-2.json), [down-both.json](../artifacts/down-both.json), restored-*.json |
| LIVE_KIND | ext-proc outage и UID mismatch после удаления старых gateway Pods | PASS | [gateway-failures-retry.log](../artifacts/gateway-failures-retry.log) |
| LIVE_KIND | direct policy revoke и unknown actor | PASS | [verify-final.log](../artifacts/verify-final.log) |
| LIVE_KIND | repeated deploy, rollback, redeploy | PASS | [verify-final.log](../artifacts/verify-final.log) |
| REPO | go test -race -p 2 ./... | FAIL | [full-tests-final.log](../artifacts/full-tests-final.log) |
| REPO | golangci-lint после исправления ST1013 | PASS | [golangci-final.log](../artifacts/golangci-final.log) |
| REPO | hack/verify-all.sh | FAIL | [verify-all-final.log](../artifacts/verify-all-final.log) |

В общем Go suite ошибки только в неизменённых `cmd/ateworker-microvm/internal/child`
(`unsupported operation` на macOS) и `internal/e2e` (среда задаёт `NO_COLOR`).
Повтор `env -u NO_COLOR go test -race ./internal/e2e` прошёл: [e2e-format-retry.log](../artifacts/e2e-format-retry.log).
Это обычный unit-прогон repository; upstream e2e suites без `--e2e` не запускают live cluster tests.

Verify-all завершён: codegen, go-modules, gofmt, licenses и proto-fmt отказали из-за dirty checkout.
Commit/stash не создавались. Единственный lint issue в новых файлах, numeric 503, исправлен;
повтор golangci-lint прошёл. Остальные verifiers, включая boilerplate, metrics, package-deps,
Python tests/licenses/protos и ShellCheck, прошли. Созданные проверками временные venv удалены.

Исправления harness, найденные живым прогоном:

- Отсутствующая actor policy возвращает 0 с пустым stdout. Bootstrap теперь создаёт её явно.
- Негативный TLS case может закрыть весь kubectl port-forward. Каждый case получает отдельную сессию; произвольная network error не считается TLS rejection.
- Restricted ephemeral container унаследовал root UID worker Pod. Probe теперь явно использует UID/GID 10001.
- rollout status не означает конец старых HTTP/2 sessions. Gateway tests ждут удаления старых Pods.
- Частичные PASS rows не маскируют exception: error marker делает этап FAIL.

Итоговый `make -C setup-vpc-proxy verify` завершён с exit 0: все 21 этап PASS.
Gate: [verify.json](../artifacts/verify.json). Отсутствующий evidence означает BLOCKED,
ненулевой stage exit означает FAIL. Секреты, private keys и Secret.data в evidence не сохраняются.

Plaintext CONNECT закрылся без новых connect_open на обоих connectors. Через валидный mTLS private/node/service/metadata IP и порт 22 получили 403; hostname authority получил 400. Эти случаи входят в 28-case matrix.
После verify gateway и оба connectors Ready 1/1; все три actors RUNNING. Остались только Docker containers собственного kind node и registry.
