# Ограничения

- IPv4-only: TCP 80/443 к публичным адресам. IPv6 запрещён. IPv4-mapped адреса проверяются после Unmap.
- Это public opaque CONNECT mode. Для actor-1/2 он обходит штатные HTTP per-request ACL, MITM, DLP и credential injection.
- EgressPolicy actor-1/2 фильтрует DNS через штатный relay, но не превращается в hostname ACL для opaque tunnel.
- Публичный literal IP доступен без DNS. Разрешённый внешний сервер сам может быть relay.
- DNS остаётся на исходном relay, а не идёт через connector.
- Нет VPC, VPN, private DNS, UDP/QUIC/ICMP, secondary interface и production controller.
- gVisor обеспечивает границу user-space kernel; это не аппаратная microVM.
- В текущем handler SVID содержит имя/atespace, но не UID. Registry UID предотвращает использование устаревшей записи после пересоздания actor, но не устраняет повторное использование старого SVID с тем же именем. Имена не переиспользуются в пределах стенда.
- Bindings статические. ConfigMap без rollout не меняет уже загруженный registry.
- Открытый tunnel не отзывается изменением файла. Для отзыва нужны revoked config и gateway rollout, при необходимости restart connector. Gateway rollout прерывает соединения других actors.
- `rollout status` не гарантирует завершение старых HTTP/2 sessions. Для проверки нового registry или ext-proc outage harness ждёт удаления всех старых gateway Pods.
- Ротация сертификатов требует согласованной остановки gateway, генерации PKI, render/deploy и проверки новых fingerprints. Автоматического SDS rotation для connectors нет.
- Локальный Envoy fixture проверяет маршрутизацию и sanitation отдельно от MITM store. Полный runtime config проверяется отдельным validation Pod с образом baseline.
- Port-forward mTLS test проверяет connector TLS, но не NetworkPolicy. Для политики есть отдельная CNI conformance и worker/actor isolation проверки.
- Ephemeral probe на реальном worker не получает actor certificates и заканчивается после проверки. Запись завершённого ephemeral container сохраняется до удаления worker Pod.

При включённом стенде NetworkPolicy gateway разрешает public IPv4 TCP 80/443, штатный DNS, ate API и kind OTLP collector, а TCP/8443 только к connectors. Это ограничение сети для трёх actors MVP; rollback удаляет обе gateway policies и возвращает baseline. Проверка EgressPolicy actor-3 продолжает выполняться.
