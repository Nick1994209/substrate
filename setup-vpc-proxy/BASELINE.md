# Baseline

Compatibility port: `94e0a26ac90151886a76410568aaac2664a54d90`, ветка `main`.
Предпочтительный SHA pack: `ed0371a15f6f265c64100001adafb9731be03970`.
Checkout не переключался. Исходные изменения: только untracked `setup-vpc-proxy-prompts/`.
Go 1.27.3 darwin/arm64, go.mod 1.27.0, toolchain go1.27.3.
Docker client/daemon 29.5.3, aarch64, VM memory 8217145344 bytes.
Исходный kubectl 1.31.0 сохранён; для стенда установлен локальный kubectl 1.37.0 с официальной SHA256. Kind 0.33.0.
Создан Kubernetes 1.37.0, Calico 3.33.0. Node и CNI Ready; enforcement проверяется отдельно.
Свободно около 23 GiB после сборок. Чтение hw.memsize запрещено sandbox.
Gateway image: `localhost:5001/atenet-f8fa82c2577202c08e487b7d6887148f@sha256:6d5c8805e6542b2c52fb1d5bc91a1964df90ee28b8b6ba1a5458d0cb4bdba996`.
Connector image: `localhost:5001/proxy-b2a61f8820faf9f81a404a7e90335ab6@sha256:f95ec42bcad05936e0649968023e6ee1b4b677321ef11574711bfbb5596490a2`.
Эти digests совпали с runtime imageID после итогового redeploy.
Pinned tooling images: см. config/kind.yaml и config/cni-images.json.
После разрешённой пользователем очистки Docker было свободно 41 GiB; все старые volumes сохранены.
Фактический handler проверяет worker-for-actor URI, состояние RUNNING/RESUMING.
ActorIdentity UID/purpose проверка в этом handler отсутствует; она не удалялась.
Для atelet добавлен hostAlias `kind-registry` с IP собственного Docker registry.
Без него golden actor не скачивал образ: cluster DNS не знает имя Docker container.
Docker credential helper зависал; scripts используют отдельную пустую Docker auth configuration.

Реальные actors в `ate-demo-egress`:

- `actor-1`: `c2f8fdf1-c611-4b50-8a76-bf7724cb402d`, `proxy`.
- `actor-2`: `ede80a2a-7d2c-4e60-a202-363607548d08`, `proxy`.
- `actor-3`: `b46e1beb-f174-4f33-a614-cb29c0d6c9a0`, `direct`.

Config SHA256: `9f705069eb4a863ff42df30bc1b0ea79a0c2bca8637c21b481384375e116abc1`.
Baseline objects SHA256: ConfigMap `d1188e329fbaa9d6986b506048099e0a523a2582028f121f76c2820342c23700`, Deployment `e4bc70794b2adb6bbec7c5ac676d1271c48599f4a8de3f77be28d8ef84cd1b90`.

Фактический Envoy baseline image: `localhost:5001/envoy-dataplane@sha256:0fafd43d51c1f1d7f6f8b84dab0bdba95cd7b16d2725318cead3cc75ab4148a8`.
Python 3.13; scripts требуют Python 3.11+. Все 21 этап итогового verify PASS; отдельно repository-wide проверки имеют ограничения из docs/verification.md.
