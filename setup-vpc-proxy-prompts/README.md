# Промпты для MVP actor-aware egress

Это задания AI coding agent, а не готовая реализация и не подтверждение работающего стенда.
Проверенная при подготовке структура Substrate: `ed0371a15f6f265c64100001adafb9731be03970`.
Дата подготовки: 2026-11-05. Основной репозиторий: https://github.com/agent-substrate/substrate

## Как запускать

1. Распаковать архив **вне исходного дерева** либо в игнорируемую папку. Агент работает в checkout Substrate.
2. Первым сообщением дать агенту весь `init_prompt.md`. Приложить ему остальные файлы архива или дать путь к ним.
3. Если агент работает поэтапно, отправлять `01_...md` … `09_...md` в указанном порядке. `init_prompt.md` уже требует прочитать их все: повторять необязательно.
4. Между этапами сохранять код и `setup-vpc-proxy/PROGRESS.md`. В новую сессию передать init_prompt и PROGRESS.
5. Не принимать ответ «всё готово» без результатов `make -C setup-vpc-proxy verify`, matrix PASS/FAIL и SHA запущенного образа.

## Целевая топология

```text
actor-1 -- штатный atunnel + actor mTLS --> atenet-egress
                                            `-- binding-1 mTLS --> proxy-vpc-pod-1 --> Internet
actor-2 -- штатный atunnel + actor mTLS --> atenet-egress
                                            `-- binding-2 mTLS --> proxy-vpc-pod-2 --> Internet
actor-3 -- штатный atunnel + actor mTLS --> atenet-egress
                                            `-- исходный EgressPolicy/MITM pipeline --> Internet
```

Без VPC, VPN, Multus, SDS/CDS, production controller и изменений actor identity protocol.
Два однорепличных Deployment/Service connectors. Имена Pod с генерируемым суффиксом нормальны:
`proxy-vpc-pod-1-<hash>-<suffix>`, `proxy-vpc-pod-2-<hash>-<suffix>`.
Вместо Squid выбран небольшой Go HTTP/1.1 CONNECT server с нативным mTLS: меньше компонентов для MVP.

## Что есть в пакете

- `init_prompt.md`: стартовая инструкция, установка kind + Substrate, ограничения работы.
- `01_contract.md`: обязательные решения, файловое дерево, security contract.
- `02_kind_substrate.md`: bootstrap, CNI с доказанной NetworkPolicy enforcement, реальные actors.
- `03_registry_egress_code.md`: Go API, загрузка registry, точные места подключения к atenet.
- `04_proxy_code.md`: обязательный Go CONNECT proxy, TLS, streaming и тесты.
- `05_envoy_pki_yaml.md`: routing, конфигурации Envoy, PKI и Service/Deployment.
- `06_deploy_automation.md`: build/render/deploy/rollback и Makefile.
- `07_tests.md`: unit/integration/e2e, отрицательные проверки и доказательства маршрутов.
- `08_security_review.md`: самостоятельный security review с критериями отказа.
- `09_handoff.md`: финальная документация и отчёт без выдуманных результатов.
- `reference/*`: конкретные Go/Envoy/YAML-контракты, которые надо реализовать и подключить; это не готовый patch.
- `SOURCES.md`: проверенные исходные пути и официальная документация.

## Важные договорённости

Весь новый самостоятельный код хранится в `setup-vpc-proxy/` внутри checkout Substrate.
Небольшие glue-изменения в `cmd/atenet/internal/router/` **обязательны**: один ConfigMap не подключает новый Go handler.
Go запрещает импортировать `cmd/atenet/internal/...` из `setup-vpc-proxy/...`; это явно учтено.
Существующий DNS relay требует EgressPolicy даже для actor-1/2; DNS не пойдёт через proxy.
У actor-1/2 opaque TCP bypass штатного HTTP MITM допускается только как явно включённый режим MVP.
Публичный доступ ограничен портами 80/443 и запретом специальных/внутренних диапазонов; по доменам он не ограничен.
actor-3 остаётся под исходным EgressPolicy; «direct» не означает «без policy».

В архиве нет ключей, сгенерированных сертификатов или секретов. Их создаёт агент локально при реализации.

## Минимальное сообщение агенту

```text
Работай в checkout agent-substrate/substrate.
Prompt pack находится в /absolute/path/setup-vpc-proxy-prompts.
Открой init_prompt.md, затем прочитай весь pack и reference.
Выполни задание, а не только составь план.
Реализация в setup-vpc-proxy/, узкий core glue diff разрешён по контракту.
Не объявляй runtime-тесты успешными без реального запуска.
```

`/absolute/path/` заменить на реальный путь после распаковки.
