# Архитектура

Схема находится в [architecture.mmd](architecture.mmd).
Первый TLS hop проходит от штатного atunnel до atenet-egress.
Envoy проверяет actor CA, очищает XFCC и передаёт цепочку в ext_proc.
Go handler повторно проверяет сертификат и получает actor через ate API.
Только RUNNING/RESUMING actor может перейти к registry.

Registry загружается один раз на startup. Ключ состоит из atespace, имени и UID,
прочитанного из текущего ответа GetActor. Ошибки не включают legacy fallback.
Actor-1 выбирает только `vpc_proxy_1`, actor-2 только `vpc_proxy_2`.
Actor-3 продолжает существующий EgressPolicy/MITM pipeline.

Второй TLS hop проходит от Envoy к Go connector по HTTP/1.1 CONNECT.
У каждого upstream свой client key/cert и CA. Proxy проверяет CA, EKU, время и единственный exact client URI SAN.
Envoy проверяет server CA и exact server URI SAN. Исходный CONNECT authority остаётся numeric origin IP:port.
CONNECT завершается в Go proxy. Только после успешного dial proxy отвечает 200 и передаёт buffered bytes в origin.

Outer Lua удаляет входящий `x-ate-vpc-cluster` до ext_proc.
Только ext_proc может поставить этот routing header. Ответ очищает route cache.
Два exact-header route предшествуют deny route для неизвестного значения и исходному direct route.
Новые mutation rules запрещают менять system headers и разрешают только этот routing header.
Inner listeners и credential-injection filters не меняются.

Применённый gateway ConfigMap прошёл `envoy --mode validate` на том же custom image.
Фрагмент `.work/rendered/envoy.yaml`, без ключей:

```yaml
name: vpc_proxy_1
type: STRICT_DNS
# load_assignment -> endpoints -> lb_endpoints -> endpoint
address:
  socket_address:
    address: proxy-vpc-pod-1.vpc-proxy.svc.cluster.local
    port_value: 8443
# transport_socket -> typed_config -> common_tls_context
tls_params:
  tls_maximum_protocol_version: TLSv1_3
  tls_minimum_protocol_version: TLSv1_3
validation_context:
  match_typed_subject_alt_names:
    - matcher:
        exact: spiffe://vpc-proxy.local/proxy/proxy-1
      san_type: URI
  trusted_ca:
    filename: /etc/vpc-proxy/client-1/ca.crt
```

Это выдержки из разных уровней одного cluster, а не самостоятельный manifest.
Второй cluster использует только client-2 и server URI proxy-2.
Полные manifests воспроизводятся через `make render`; Secret manifests остаются в private `.work`.

Live smoke подтвердил CONNECT delta `[1,0]`, `[0,1]`, `[0,0]` соответственно для actors 1, 2, 3.
В gateway JSON есть настоящий `actor_uid`; connector JSON содержит exact `peer_uri` и numeric `destination`.
