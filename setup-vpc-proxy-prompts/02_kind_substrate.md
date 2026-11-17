# PROMPT 02 — bootstrap настоящего kind + Substrate

Создай автоматизацию и **выполни** запуск. Не подменяй его YAML-планом.

## Preflight

Проверить Docker daemon, архитектуру, свободные ресурсы, порты local registry, kubectl, Go toolchain из go.mod, сетевую доступность image registry. Версии инструментов фиксировать. Не загружать случайные latest бинарники и не понижать зависимости.
Все scripts используют set -Eeuo pipefail, явный context, bounded waits и trap для временных port-forwards. Вывод секретов и set -x на PKI запрещены.
Свой context: `kind-substrate-vpc-proxy`. Проверка имени + маркера владения перед cleanup.

## CNI

Предпочтительно создать новый kind с `disableDefaultCNI: true` и установить один совместимый pinned policy-enforcing CNI, например Calico. **Не ставь два конкурирующих default CNI.**
Сгенерируй kind config в `.work/kind.yaml`, беря необходимые Kubernetes version/feature gates/runtimeConfig/registry containerd настройки из текущего upstream `hack/create-kind-cluster.sh` и pinned tooling. Не копируй старый произвольный kind YAML, не теряй PodCertificateRequest и ClusterTrustBundle, необходимые Substrate.
Для проверенного baseline нужны feature gates ClusterTrustBundle, ClusterTrustBundleProjection, PodCertificateRequest и certificates.k8s.io/v1beta1; на другом SHA сначала проверь upstream.
KVM необязателен для gVisor. Сохранить upstream node sysctl/loopback networking/runtime install prerequisites; выяснить их из актуального скрипта, а не угадывать.
Реализуй upstream-equivalent local registry wiring, отдельный reusable bootstrap script. Не изменяй upstream hack/create-kind-cluster.sh и не вызывай его destructive delete при повторном запуске.
Можно использовать upstream script на новом имени только если обеспечен выбранный CNI и не потеряны гарантии; итоговые шаги документировать. Пересоздание существующего собственного кластера — только с явным RESET_CLUSTER=1.

Выбери CNI release по совместимости с реальной Kubernetes version, фиксируй version/digest и официальный источник manifests. Сначала дождись CNI/node Ready, затем Substrate. Если CNI конфликтует с gVisor/atunnel networking, исправляй конфигурацию или отмечай blocker, не отключай защиту.

## Установка

Реализуй команды из init_prompt с `ATE_INSTALL_KIND=true` в **каждом** дочернем запуске ate-setup. Явно передавай context и registry settings согласно актуальному --help.
Используй `--credential-provider='{"enabled":false}'`: MVP не требует секретов из Kubernetes Secrets.
Дождись Ready системных компонентов и рабочего egress demo до модификаций. Не переустанавливай baseline поверх custom overlay на каждом make deploy.

Изучи:
- demos/egress/README.md, egress.yaml.tmpl, egress-template.yaml.tmpl;
- hack/verify-egress-demo.sh;
- cmd/ate-setup/internal/demos/egress/egress.go;
- CLI `kubectl ate create actor`, `get actor`, `create/update egress-policy`, `proxy actor`.

Создай `actor-1`, `actor-2`, `actor-3` в `ate-demo-egress` через ate API/CLI с шаблоном egress.
Стандартный egress demo Actor принимает POST JSON `{"url":"https://example.com/"}` — используй реальную реализацию из checkout. Добавлять свой actor-probe можно только если существующий fixture не покрывает тест, весь новый код тогда в setup-vpc-proxy/cmd/actor-probe.
У WorkerPool должно быть достаточно активных slots для трёх actors одновременно (минимум 3), чтобы concurrent tests не зависели от oversubscription. Фиксируй фактическую конфигурацию.
Не делай `kubectl apply` для protojson-shaped ActorTemplate: это ate API resource, не автоматически Kubernetes CRD. WorkerPool — Kubernetes CRD. Следуй реальному CLI.
Считай server-generated UID каждого actor, проверь непустые значения и сформируй routes YAML; не используйте UUID-заглушки.

## Policy и DNS

На всех трёх actors явно создать EgressPolicy на выбранные публичные тестовые имена. Иначе DNS relay вернёт NXDOMAIN **до нового gateway hook**.
Предпочтительный target: `https://example.com/` на TCP/443 и дополнительный http target только для smoke/streaming. Реальную доступность endpoints проверять, не записывать заранее «200».
Для actor-3 удобно `tls_passthrough` правило, чтобы сохранить origin TLS без добавочных MITM изменений. YAML field casing/CLI взять из реальной API schema и существующего egress demo. Для actor-1/2 та же policy нужна DNS, но не является HTTP enforcement на opaque proxy route.
Нельзя отключать DNS policy globally или выдавать `*` всем actors ради удобства.

## Проверки

1. Реальный gVisor actor baseline fetch до изменения gateway.
2. NetworkPolicy conformance: созданный в отдельном test namespace server достижим до deny, недостижим после deny, точечный allow возвращает доступ. Иначе security gate FAIL.
3. Реальная невозможность actor/worker прямо достичь connector адресов после deploy — проверяется отдельно на финальном этапе.

Сохрани baseline SHA, tool versions, Kubernetes/CNI versions, command logs и фактические node/runtime сведения в artifacts. Если среда ограничена — BLOCKED, а не simulated PASS.
