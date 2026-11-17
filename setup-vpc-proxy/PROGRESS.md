# Ход работ

Прочитаны init, 01–09 и reference. Реализован compatibility port на
`94e0a26ac90151886a76410568aaac2664a54d90`; checkout не переключался.

Выполнено:

1. Docker Desktop запущен. По запросу пользователя удалены старые контейнеры, images и build cache; volumes сохранены. Последующие контейнеры и images созданы для этого стенда и проверок.
2. Созданы отдельный kind Kubernetes 1.37.0, Calico 3.33.0, registry и kubeconfig. CNI enforcement allow/deny/allow PASS.
3. Установлены настоящий Substrate, custom Rust Envoy, gVisor workers, egress demo и golden snapshot. Исправлены DNS registry и зависший Docker credential helper без изменения host configuration.
4. Созданы actor-1/2/3 с реальными UID. Baseline fetch PASS. Исправлен CLI case отсутствующей policy с exit 0 и пустым stdout.
5. Реализованы строгий registry и authenticated CONNECT hook после state check, opt-in flag и startup loader. Direct actor сохраняет EgressPolicy.
6. Реализованы mTLS CONNECT proxy, public target policy, buffered streaming, half-close, resource limits, health и shutdown.
7. Реализованы PKI, semantic Envoy renderer, NetworkPolicy, Secrets и validation Pod. Полный конфиг прошёл actual Envoy validation и применён.
8. Live smoke/concurrency PASS: actors 1/2 используют только свои connectors, actor-3 direct. Gateway UID, connector URI и numeric destination сопоставлены.
9. mTLS 12 cases, literal IPv4, actor/worker isolation и раздельные scale-to-zero PASS. После восстановления proxies fresh fetch всех actors PASS.
10. Ext-proc outage и UID mismatch PASS после ожидания удаления старых gateway Pods. Policy revoke и unknown actor PASS в отдельных прогонах.
11. Repeat deploy PASS: UID actors, fingerprints CA/client/server, UID/count connector Deployments и config hash совпали.
12. Целевые count=3/shuffle/race, custom Envoy integration и golangci-lint PASS. ShellCheck и Python harness tests PASS.
13. Независимые security и adversarial review выполнены. Подтверждённые замечания исправлены, последний reviewer перепроверил evidence classifiers без новых findings.
14. Общий repository suite выполнен: FAIL в неизменённых microVM child tests на macOS и цветных formatting tests с NO_COLOR. Formatting retry без NO_COLOR PASS. Verify-all выполнен; dirty-tree guards FAIL, единственный новый lint issue исправлен и повторная проверка PASS.

Итоговый `make -C setup-vpc-proxy verify` завершён с exit 0. Все 21 этап PASS, включая 28 TLS/destination/plaintext cases, repeat, rollback и redeploy.
Gateway и оба connectors Ready 1/1; три actors RUNNING. Незавершённых обязательных этапов MVP нет.
Cold worker pull timeout теперь имеет один bounded retry; полное пересоздание кластера после этой последней bootstrap-правки отдельно не запускалось. ShellCheck PASS.
Фактическая итоговая matrix: `artifacts/verify.json`, лог: `artifacts/verify-final.log`.
Секреты и kubeconfig остаются в ignored private `.work`; evidence не содержит Secret.data/private keys.
Commits, DCO, issue и PR не создавались.
