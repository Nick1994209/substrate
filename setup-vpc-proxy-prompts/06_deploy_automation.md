# PROMPT 06 — сборка, render, deploy и воспроизводимость

Реализуй рабочий Makefile и scripts. Каждый target должен выполнять действия, а не echo псевдокоманд.

## Targets

```text
preflight           версии, права, ресурсы, context, prerequisites
bootstrap           idempotent kind + CNI + Substrate + baseline egress demo
actors              три actor и EgressPolicy; считать реальные UID
pki                 создать PKI/Secrets manifests без выкладывания ключей в git
build               собрать proxy image и ИЗМЕНЁННЫЙ cmd/atenet image
render              реальные UID+image digests+конфиги -> .work/rendered
validate            Go checks + Envoy validation + manifests dry-run
deploy              bootstrap prerequisites, secrets, proxies, gateway overlay + wait
smoke               три настоящих actor-запроса и маршруты
verify              unit/integration + policy + mTLS + e2e + fail-closed
logs                безопасные filtered logs по трём actors
collect-evidence    version/image/config hashes, logs, results без Secret data
rollback            восстановить original gateway objects и удалить custom resources
cleanup             custom objects; кластер удалить только при DESTROY_CLUSTER=1
```

`make -C setup-vpc-proxy all` можно определить как bootstrap actors pki build render validate deploy smoke.
Все paths вычисляй от git repo root, не от текущего cwd. Используй namespace/context в каждом kubectl.
Не требуй повторно передавать ATE_INSTALL_KIND в ручной shell: common.sh явно экспортирует его в нужных вызовах.

## Build images

Proxy image из setup-vpc-proxy/cmd/proxy. Gateway ext-proc image — из существующего `./cmd/atenet` после glue changes; новый независимый proxy image не заменяет atenet binary.
Предпочтительно использовать существующий pinned ko toolchain (`hack/run-tool.sh ko ...`, проверив --help и repository tooling). Нельзя забыть root Go module/internal imports. Можно новый Dockerfile, но build context — repo root.
`KO_DOCKER_REPO` должен соответствовать фактическому kind registry; version/digest зафиксировать. Избегать latest и неопределённых image tags. Root module Go version и pinned build platform соблюдать.
Собранный digest реально должен стоять в gateway container `ext-proc`. Envoy container image/version baseline не менять без отдельной причины.

## Render

cmd/render — semantic YAML parser, не sed по многострочному Envoy тексту.
Считать сохранённый исходный ConfigMap/Deployment из baseline; раскрыть data.envoy.yaml; найти outer listener/HCM; изменить маршруты/фильтры/clusters строго по expected shape. На повторном render не добавлять дубликаты.
Исходный manifests/ate-install/atenet-egress.yaml может быть template/install overlay input; не считать его уже окончательным runtime config.
Для ConfigMap/Deployment snapshot сохранять только declarative fields, без resourceVersion/uid/status/managedFields и секретных данных. Backup не перезаписывать custom-версией при втором deploy.
Сохранить `.work/baseline.sha`, baseline object hashes и `.work/state.json` с именами/UID/image refs. Перед повторным deploy проверить соответствие кластера/checkout; не deploy stale actor config.
Secrets render только в private .work под umask 077. Config hashes не должны содержать private key в log.
Разделить K8s YAML и ate API YAML. Для K8s apply/dry-run, для actors/policies — ate CLI.

## Порядок deploy

1. Убедиться, что baseline и CNI живы; custom registry пока не включён.
2. Создать actors и policies и считать UID.
3. Создать локальные сертификаты и public policy configuration.
4. Создать proxy namespace/deny policies/Secrets/Deployment/Services и дождаться Ready.
5. Сохранить original gateway configuration один раз.
6. Выполнить render + validation с actual images/certs.
7. Применить новые client Secrets, routes ConfigMap и Envoy ConfigMap.
8. Применить Deployment overlay одним согласованным шагом: changed image, flag, mounts, checksum annotations.
9. Дождаться rollout gateway и smoke всех actors.

Во время изменения config предусмотреть fail-closed interruption, а не временный direct bypass.
Proxy Deployments могут restart независимо; gateway на new CONNECT не должен иметь route fallback.

## Repeat/rollback

Повторный `make deploy` не должен плодить actors, CA, proxies или clusters; сравнить UID, resource count, cert fingerprint.
Но actor UID mismatch — явная ошибка, не молчаливая перепривязка. Для recreating fixtures нужен отдельный explicit reset path с учётом ограничения name reuse.
`rollback` возвращает original gateway ConfigMap/Deployment и ожидает rollout; удаляет feature-owned NetworkPolicies/Secrets/proxies, не трогая другие проекты. Объясни: rollback намеренно выключает feature, поэтому к нему нельзя обращаться как к автоматическому fallback при proxy outage.
Cleanup должен удалять созданные actor resources через ate API, не только namespace. Не выполнять go run ate-setup delete all на чужой установке.

Готовность: одна команда с clean baseline доводит до smoke; повторный deploy idempotent; rollback протестирован на собственном стенде. BLOCKED если инструментов/прав нет, без fake evidence.
