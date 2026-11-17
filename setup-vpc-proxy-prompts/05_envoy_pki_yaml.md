# PROMPT 05 — PKI, Envoy wiring и Kubernetes manifests

Реализуй cmd/pki и cmd/render, templates и проверку готовой конфигурации той же версией Envoy, что реально запущена в baseline. Нельзя заменить весь envoy.yaml сокращённым примером.

## PKI

`make pki` создаёт local development certificates внутри .work/pki под umask 077; ключи gitignored, private permissions, никаких реальных ключей в git или stdout.
Для каждого proxy отдельная private development CA (предпочтительно), отдельный gateway client key/cert и server key/cert.

| Binding | Client URI SAN | Server URI SAN | Server DNS SAN |
|---|---|---|---|
| 1 | spiffe://vpc-proxy.local/egress/proxy-1 | spiffe://vpc-proxy.local/proxy/proxy-1 | proxy-vpc-pod-1.vpc-proxy.svc.cluster.local |
| 2 | spiffe://vpc-proxy.local/egress/proxy-2 | spiffe://vpc-proxy.local/proxy/proxy-2 | proxy-vpc-pod-2.vpc-proxy.svc.cluster.local |

Client cert: EKU clientAuth. Server cert: EKU serverAuth. Не выдавать wildcard identity, сертификаты с обеими ролями без причины или CA=true leaf.
Монтировать gateway-client-1 Secret только в gateway Envoy; gateway-client-2 аналогично. Proxy server Secret включает server key/cert + доверенный client CA certificate. CA private key никогда не класть в Kubernetes. Client key не монтировать в proxy и наоборот. В ext-proc sidecar клиентские private keys тоже не нужны.
Короткий срок leaf для dev, срок отражать в доках; повторный make pki не должен молча заменить рабочие ключи. Явный rotate target делает согласованный rollout gateway/proxy и объясняет interruption. Для MVP без SDS допустим restart при rotation.

## Envoy route patch

Открой реальную ConfigMap atenet-egress после baseline install. Найди **outer listener egress**, его HCM, route_config и ext_proc. Проверить expected names/counts; если shape неизвестен — renderer fail с объяснением. Не патчить MITM внутренние listeners.

1. Перед существующим ext_proc добавить header sanitation, например Lua, удаляющую `x-ate-vpc-cluster` из клиентского CONNECT. Не нарушать текущие identity set_filter_state фильтры.
2. Сохранить `forward_client_cert_details: SANITIZE_SET`, chain forwarding и существующую проверку actor CA.
3. Добавить два **статических, exact-header** маршрута до legacy route (reference/envoy-fragments.yaml). Это проще и безопаснее, чем unrestricted cluster_header для двух known bindings.
4. После них маршрут на наличие неизвестного x-ate-vpc-cluster → direct_response 403. Нельзя позволить unknown header упасть в legacy direct.
5. Существующий route CONNECT→mitm_internal остаётся для direct, с connect_config:{} как в baseline.
6. Для proxy routes CONNECT **без connect_config**: Envoy пересылает CONNECT запрос по mTLS в Go proxy, который его терминирует и dial-ит origin.
7. У proxy upstream clusters использовать явный HTTP/1.1, отдельные client certs, доверенные CA, exact server URI SAN. SNI фиксированный DNS Service, не из actor destination authority. Не использовать auto_sni от :authority.
8. Ext_proc остаётся failure_mode_allow:false для всех маршрутов, без per-route отключения. Ответ proxy hook ClearRouteCache:true; проверь, что конфигурация не запрещает это настройками route cache. Не добавляй перед ним auth с решением по cached route.
9. Не снимать запрет upstream private targets globally: gateway разрешается лишь TCP/8443 на два connector Services/Pods средствами сетевых policy.
10. Не использовать origin IP как адрес proxy cluster. CONNECT :authority=origin-IP:port сохраняется до Go proxy.

У Lua и остальных нужных filters проверить наличие в actual Envoy image через validation. Если Lua не скомпилирован, реализуй эквивалентную supported header-mutation sanitation и обоснуй, а не отключай защиту.

## Обязательный manifests набор

- Namespace vpc-proxy, два Deployment `proxy-vpc-pod-1`, `proxy-vpc-pod-2`, replicas=1, strategy Recreate;
- два ClusterIP Service с уникальными selector binding-id/proxy-id, порт 8443;
- отдельные server TLS Secrets;
- gateway client TLS Secrets в ate-system, два readOnly mount в **envoy container**;
- ConfigMap routes только для ext-proc и флаг --vpc-proxy-config=/etc/vpc-proxy/routes.yaml;
- custom compiled ./cmd/atenet image только в **ext-proc container** gateway;
- rendered Envoy ConfigMap + rollout annotation/hash;
- ServiceAccount tokens connectors отключить; no hostNetwork/hostPID/hostPath/docker.sock/privileged;
- securityContext non-root, drop ALL, readOnlyRootFilesystem, RuntimeDefault; requests/limits;
- readiness через cmd/proxy --check-ready, liveness с bounded timeouts;
- NetworkPolicy default-deny ingress и egress в vpc-proxy;
- ingress 8443 лишь от конкретных atenet-egress pods в ate-system, не всего namespace;
- connector public egress только TCP 80/443 с explicit special/private/cluster deny ranges;
- gateway дополнительный egress allow только на proxy pod selectors + 8443, не весь RFC1918;
- metrics/health не публиковать снаружи.

## NetworkPolicy тонкости

В Kubernetes policies аддитивны: default deny не отменяет широкие allow. Проанализируй итоговые upstream policies и убедись, что worker/gateway/proxy ограничения не перекрыты широкой policy.
Public CIDR policy должна учитывать IPv4/IPv6: если реализация IPv4-only, не добавляй IPv6 allow. Origin IP проверяется ещё раз в Go proxy. DNS proxy не нужен — CONNECT numeric. Gateway DNS discovery Service разрешён upstream, сохрани его.
На kind Node/host traffic имеет особенности policy enforcement: не обещай то, что не проверено. На этом MVP нет secondary interface, VPN или hostNetwork.

## Валидация

- `envoy --mode validate -c ...` в точном deployed image, с реальными Secret mounts/подготовленными путями, не пустыми файлами.
- `kubectl apply --dry-run=server` для K8s manifests (не ate API ресурсов).
- Проверить примонтированные paths в Envoy и ext-proc, actual args, Envoy config_dump только через loopback/port-forward, checksum image/config.
- Отрицательный тест TLS certificate mismatch не должен переходить в plaintext.

Результат: render produces complete executable YAML без ${...} и UUID placeholders; исходные templates сохраняются отдельно.

## Защита outer header mutations

В outer ext_proc оставить `disallow_system: true`, `disallow_is_error: true`. Строгое усиление для MVP: `disallow_all: true` плюс `allow_expression: {google_re2: {}, regex: "^x-ate-vpc-cluster$"}`. Проверить семантику именно deployed Envoy; allow_expression по HeaderMutationRules имеет приоритет над disallow_all, кроме disallow_expression. Никакие :authority/:method/:scheme/XFCC не должны стать произвольно изменяемыми. Не применять новые mutation rules к inner credential-injection filters. В baseline direct response header mutations outer hook отсутствуют, поэтому это усиление не должно ломать legacy route; доказать regression tests.
