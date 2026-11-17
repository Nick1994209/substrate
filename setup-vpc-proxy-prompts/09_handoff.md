# PROMPT 09 — передача runnable MVP

Закончить не обещанием, а воспроизводимой инструкцией и evidence.

Обязательные документы:

1. README.md: prerequisites, точный baseline, commands `make bootstrap actors pki build render validate deploy smoke verify`, пример JSON fetch для каждого actor и cleanup/rollback.
2. integration-map.md: таблица «файл/символ → что добавлено → кто вызывает → чем тестируется». Явно перечислить каждый core diff вне setup-vpc-proxy.
3. BASELINE.md: upstream SHA, toolchains, kind/Kubernetes/CNI, image digests, actual runtime, известные deviations.
4. docs/architecture.mmd + docs/architecture.md: Mermaid diagram и отдельное описание двух TLS hops. Actor3 без connector. Привести concrete deployed YAML fragments без Secret keys.
5. docs/verification.md: PASS/FAIL/BLOCKED matrix с командами и ссылками на sanitized evidence. Разделить unit/local integration/live kind.
6. docs/limitations.md и SECURITY.md: public opaque TCP vs baseline direct policy, DNS, IPv4 support, name-reuse limitation, статические bindings и ротация, open TCP revoke.
7. PROGRESS.md: что фактически выполнено и что осталось.

В финальном ответе AI agent должен указать:
- какие файлы и core hooks реализованы;
- команды запуска из чистого clone;
- какой контекст/namespace использовать;
- реальные UID actors и image/config hashes (без secrets);
- результаты раздельного scale-to-zero теста proxies;
- результаты mTLS exact URI tests и CNI enforcement;
- честные BLOCKED шаги, если kind/Docker отсутствовали.

Нельзя писать «MVP работает» если был только go test. Нельзя выдавать «созданы YAML» за применённые resources.
Если требуется archive исходников, не включать .work/pki, private keys, kubeconfig, .env, raw Secret manifests и credentials. Оставить templates и команды генерации.
