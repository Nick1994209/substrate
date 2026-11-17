# Actor-aware egress MVP

Расширение предназначено для локального стенда. Подтверждённый статус проверок находится в [docs/verification.md](docs/verification.md).
Успешные unit tests сами по себе не подтверждают работу Kubernetes или actor isolation.
Фактический snapshot UID, image/config hashes, route excerpt и 21 PASS stage сохранён в [docs/deployed-state.json](docs/deployed-state.json).

Текущий порт: `94e0a26ac90151886a76410568aaac2664a54d90`.
Нужны Go 1.27.3, Docker Desktop/Engine, kubectl, Python 3.11+, curl, openssl и `rg`.
Версии kind и ko берутся из upstream `hack/tools`. Не понижайте go.mod.
Для Docker нужно достаточно памяти и свободного диска. Этот стенд использует только gVisor.
Scripts используют отдельный `.work/docker/config.json` без credential helper и endpoint текущего Docker context.
Для registry, требующего авторизации, передайте путь к собственной конфигурации через `VPC_PROXY_DOCKER_CONFIG`.
Настройки Docker Desktop не изменяются.

Из корня checkout:

```sh
make -C setup-vpc-proxy bootstrap
make -C setup-vpc-proxy actors pki build render validate deploy smoke
make -C setup-vpc-proxy verify
```

`bootstrap` создаёт отдельный кластер `substrate-vpc-proxy`, устанавливает Calico,
проверяет NetworkPolicy, затем устанавливает Substrate и egress demo.
Если cold image pull превышает upstream demo timeout, bootstrap ждёт worker rollout до 20 минут и повторяет demo один раз. Остальные ошибки завершают команду.
В дочерние ate-setup передаются `ATE_INSTALL_KIND=true`, `--no-dev-env`,
`--context kind-substrate-vpc-proxy`, выключенный credential provider и `--sandbox-classes=gvisor`.
Существующий чужой кластер с таким именем не удаляется.
Явный `RESET_CLUSTER=1 make -C setup-vpc-proxy bootstrap` пересоздаёт только кластер с нашим UID-маркером и архивирует его локальное состояние.

Kubeconfig хранится только в `.work/kubeconfig`. Для ручных команд:

```sh
export KUBECONFIG="$PWD/setup-vpc-proxy/.work/kubeconfig"
kubectl --context kind-substrate-vpc-proxy -n ate-system get pods
setup-vpc-proxy/.work/bin/kubectl-ate --context kind-substrate-vpc-proxy get actor actor-1 -a ate-demo-egress -o json
setup-vpc-proxy/.work/bin/kubectl-ate --context kind-substrate-vpc-proxy proxy actor actor-1 18001:80 -a ate-demo-egress
```

В отдельном терминале:

```sh
curl --fail-with-body -H 'Content-Type: application/json' \
  --data '{"url":"https://example.com/","http1":true}' http://127.0.0.1:18001/
```

Аналогично actor-2 и actor-3. Проверяйте `statusCode`, непустой `body` и отсутствие `error` в JSON.
URL smoke fixtures находятся в `config/targets.json`.
Параметр `http1` создаёт отдельный transport в текущем demo, чтобы запрос не использовал старый origin tunnel.
Actors находятся в atespace `ate-demo-egress`. Connectors находятся в Kubernetes namespace `vpc-proxy`.
Gateway находится в `ate-system`. HTTP_PROXY/HTTPS_PROXY в actors не задаются.

`make pki` не заменяет существующие сертификаты. Leaf живёт 24 часа, CA 7 дней.
`make -C setup-vpc-proxy rotate` останавливает gateway и оба connectors, архивирует PKI в private `.work/archives`, выпускает новые сертификаты и выполняет deploy. При ошибке gateway остаётся остановленным; автоматического восстановления старых ключей нет.
Ключи создаются с private permissions и не попадают в git или evidence.
Сборка по умолчанию использует upstream ko и корневой `.ko.yaml`.
Альтернативный multi-stage Dockerfile использует корневой build context; его entrypoint `/proxy` отличается от ko `/ko-app/proxy`.
Для альтернативной сборки нужно согласованно заменить probe command в manifest.

```sh
make -C setup-vpc-proxy collect-evidence
make -C setup-vpc-proxy rollback
make -C setup-vpc-proxy cleanup
DESTROY_CLUSTER=1 make -C setup-vpc-proxy cleanup
```

Rollback намеренно выключает feature и восстанавливает исходный gateway.
Он не вызывается автоматически при отказе connector. Cleanup удаляет созданные actors через ate API.
Кластер удаляется только при `DESTROY_CLUSTER=1` и совпадении UID-маркера.
Имена удалённых actors нельзя повторно использовать в том же стенде.
Не передавайте `.work`, kubeconfig, Secret manifests или private keys вместе с исходниками.

Изменения выполнены с помощью AI: реализация Go, wiring, scripts, tests и документация.
Commits, DCO sign-off, issue или PR от имени пользователя не создавались.
