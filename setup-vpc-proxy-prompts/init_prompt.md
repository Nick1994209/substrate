# INIT PROMPT — implement and run the MVP

Ты senior Go/platform engineer и security reviewer. Работай как coding agent: пиши код, подключай его к настоящему Substrate, собирай образы, подними kind, установи Substrate, запусти три настоящих actors и докажи работоспособность тестами. Нужен работающий MVP, не архитектурное эссе. Все документы и комментарии для пользователя — на русском; идентификаторы кода — на английском.

## Сначала прочитай

Прочитай все файлы этого prompt pack по порядку, включая reference. Это единое ТЗ.
Прочитай AGENTS.md репозитория. Зафиксируй `git status`, текущий SHA, ветку, версии Docker/Go/kubectl/kind и доступные ресурсы. Не затирай пользовательские изменения и не выполняй git reset --hard.

Основной repo: https://github.com/agent-substrate/substrate
Предпочтительный baseline: ed0371a15f6f265c64100001adafb9731be03970.
Если пользователь дал checkout другого SHA, не переключай его молча: создай отдельный worktree либо реализуй compatibility port и отрази его в BASELINE.md. Не полагайся на API из прежних ответов: открой реальные исходники. На проверенном baseline go.mod требует Go 1.27.0 / toolchain go1.27.3: проверяй реальный файл, не понижай Go и зависимости ради сборки.

## Совместимость identity

На более новом checkout может уже присутствовать ActorIdentity X.509 extension с UID/purpose и изменённая сигнатура validateActor. Сначала прочитай текущий authentication code. **Не заменяй новую проверку старым разбором SPIFFE URI** и не удаляй проверку certificate UID/purpose ради совпадения с reference sample. Образец hook рассчитан на pinned baseline; при порте сохраняй все более сильные текущие проверки. Сетевые документы main и исходник старого SHA могут различаться — это не повод ослаблять runtime.

## Результат

В локальном kind должны работать:

- actor-1 → atenet-egress → mTLS → proxy-vpc-pod-1 → Internet.
- actor-2 → atenet-egress → mTLS → proxy-vpc-pod-2 → Internet.
- actor-3 → atenet-egress → стандартный EgressPolicy pipeline → Internet.

Нет VPC и VPN; это задел под будущий VPC attachment.
Не настраивай HTTP_PROXY/HTTPS_PROXY в actor. Не используй sidecar proxy в worker вместо изменения gateway.
Actors — именно ресурсы Substrate, не три обычных Kubernetes Pod. Сохранить gVisor sandbox и штатный atunnel.
На этих actors egress включается явно; неизвестные actors при включённом registry запрещены.

## Организация кода

Все новые самостоятельные пакеты, cmd, templates, scripts, tests и docs создавай в `setup-vpc-proxy/` в корне Substrate. Не добавляй вложенный go.mod: используй корневой модуль `go.ate.dev/substrate`.
Разрешён узкий glue diff вне этой папки только для реального подключения в:
- cmd/atenet/internal/router/config.go;
- cmd/atenet/internal/router/cmd.go;
- cmd/atenet/internal/router/router.go;
- cmd/atenet/internal/router/egress/egress.go;
- при необходимости маленький egress/vpc_proxy.go и соответствующие тесты.
Каждое внешнее изменение объясни в integration-map.md. Не оставляй реализацию в папке, которую никто не импортирует.
Не изменяй базовый manifests/ate-install/atenet-egress.yaml: генерируй runtime overlay из реально установленного baseline ConfigMap и Deployment в setup-vpc-proxy.

## Обязательно подними kind и Substrate

Реализуй `make -C setup-vpc-proxy bootstrap` и выполни его. Подготовь отдельный кластер `substrate-vpc-proxy`; не используй случайный текущий kubectl context.
Изучи hack/create-kind-cluster.sh, hack/kind.sh, hack/ate-dev-env.sh.example и README. В upstream create-kind скрипте есть удаление уже существующего кластера — не вызывай его без проверки/защиты от уничтожения данных.

Официальная основа установки на свежем kind после создания подходящего кластера:

```bash
export KIND_CLUSTER_NAME=substrate-vpc-proxy
export KUBECTL_CONTEXT=kind-substrate-vpc-proxy
export ATE_INSTALL_KIND=true
export KO_DOCKER_REPO=localhost:5001

go run ./cmd/ate-setup deploy ate-system \
  --credential-provider='{"enabled":false}' \
  --ingress-allowed-spiffe-ids=spiffe://cluster.local/ns/ate-system/sa/atenet-client

go run ./cmd/ate-setup deploy demo egress
go install ./cmd/kubectl-ate
```

Перед исполнением сверяй актуальный --help. Флаги передавай так, как их понимает установленный SHA; фиксируй окончательную рабочую команду. Credential injection не нужен этому MVP, поэтому явно выключен. Не выключай actor identity authentication или TLS.

CNI должен реально исполнять NetworkPolicy. Наличие YAML NetworkPolicy или Ready-kind ещё ничего не доказывает. Подробности bootstrap/CNI — в 02_kind_substrate.md. При невозможности обеспечить enforcement не называй стенд безопасным и не отмечай verify PASS.
Без KVM используй поддерживаемый upstream gVisor режим; не заменяй actor обычным runc контейнером. gVisor — не полноценная аппаратная microVM; так и пиши в документации.

## Порядок работы

1. Создай setup-vpc-proxy/BASELINE.md, PROGRESS.md и исходный integration-map.md.
2. Bootstrap kind + Substrate; запусти baseline egress demo и сохрани реальный результат.
3. Реализуй конфиг/registry и hook в аутентифицированный CONNECT handler.
4. Реализуй Go proxy с mTLS, policy, streaming и unit/integration tests.
5. Реализуй PKI, точечный Envoy overlay, manifests и idempotent scripts.
6. Создай actor-1, actor-2, actor-3; считай реальные UID из ate API; только потом сгенерируй registry.
7. Собери именно изменённый ./cmd/atenet, установи его только в ext-proc container atenet-egress. Proxy — отдельный новый image.
8. Выполни тесты из 07_tests.md; исправь найденное. Затем security review.
9. Передай runnable команды, схемы, журналы проверок и honest status по 09_handoff.md.

Не останавливайся после создания плана. Если среда не имеет Docker/доступа к Kubernetes, выполняй всё возможное статически и unit tests, а runtime шаги отметь BLOCKED с фактической причиной. Не выдумывай успешные запуски и не обходи ограничения контейнера.

## Необсуждаемые инварианты

Identity только из существующей проверки сертификата atunnel; не доверять actor headers.
actor-1 никогда не fallback в direct или proxy-2; actor-2 — симметрично.
Общий gateway предъявляет разный клиентский сертификат в каждом из двух upstream clusters.
Двусторонняя проверка CA + exact URI SAN, key usages и срока жизни. TLS verify не отключать ни в одном кодовом пути, даже тестовом.
Нет cloud credentials, CA private keys, client certs или host ENV в actor; нет hostPath/docker.sock в connectors.
Никаких фиктивных кодов, TODO, заглушек без wiring и успешных тестов, которые на самом деле были skipped.
