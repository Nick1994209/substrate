# PROMPT 04 — небольшой Go CONNECT proxy с обязательным mTLS

Реализуй `setup-vpc-proxy/cmd/proxy` и `pkg/connectproxy` как реальную конечную HTTP/1.1 CONNECT точку. Не добавляй Squid/Envoy sidecar в connector для MVP: TLS и tunnel делает Go server.

## CLI contract

Обязательны flags:

```text
--listen=:8443
--tls-cert=/etc/proxy/tls/tls.crt
--tls-key=/etc/proxy/tls/tls.key
--client-ca=/etc/proxy/tls/ca.crt
--allowed-client-uri=spiffe://vpc-proxy.local/egress/proxy-1
--proxy-id=proxy-vpc-pod-1
--max-tunnels=128
--dial-timeout=5s
--tunnel-idle-timeout=60s
--max-tunnel-duration=5m
--health-listen=127.0.0.1:9090
```

proxy-2 отличается proxy-id/allowed-client-uri/Secrets.
`--check-ready` отдельный режим того же бинарника: TCP dial loopback health listener и проверка status/response; не требует shell/curl в distroless. Health listener не предоставляет CONNECT и не публикуется Service.

## TLS

Используй tls.Config:

```go
MinVersion: tls.VersionTLS13,
ClientAuth: tls.RequireAndVerifyClientCert,
ClientCAs: configuredCAPool,
NextProtos: []string{"http/1.1"},
```

Подпись/цепочка/EKU/время проверяются штатным TLS. В VerifyConnection **после стандартной проверки** проверить ровно один ожидаемый URI SAN leaf; нельзя принимать любой URI того же trust domain. Ошибка → abort handshake. Не заменять штатную verification собственным вызовом без нужных x509.VerifyOptions.
Используй reference/mtls.go.txt как обязательный принцип. Не использовать InsecureSkipVerify, даже для тестов. Tests должны устанавливать RootCAs/ServerName.
Два разных ключа клиента на gateway, два server keys на proxies, желательно отдельная CA на binding. Дополнительные wrong-URI cert tests генерировать локально test fixture CA, а не изменять runtime trust.

## HTTP/CONNECT

- Поддерживать только метод CONNECT; остальные 405, кроме отдельного loopback health.
- CONNECT authority — строго literal numeric IPv4/IPv6 + port с `net.SplitHostPort`, затем `netip.ParseAddr`, нормализация Unmap. Hostnames запрещены: proxy не должен выполнять DNS и создавать rebinding vulnerability.
- Проверить r.Host / r.URL.Host / method/RequestURI на непротиворечивость; reject userinfo/path/query/fragment/некорректные bracketed IPv6. Нельзя интерпретировать URL из actor payload.
- Отказать CONNECT с body/Transfer-Encoding: не позволять неожиданную HTTP framing семантику.
- Проверить тот же public target policy, что gateway. `IsGlobalUnicast()` НЕ достаточен: RFC1918/CGNAT и специальные диапазоны необходимо исключить явно.
- Зафиксируй conservative public policy для MVP: deny private, loopback, link-local, unspecified, multicast, IPv4-mapped private IPv6, CGNAT, benchmark/documentation/reserved networks и инфраструктурные CIDR kind. Для IPv6 либо обеспечь корректные special-use deny ranges с tests, либо **явно ограничь MVP IPv4-only и запрети IPv6**. Нельзя молча называть IPv4-only «полный Internet».
- Ports по умолчанию строго 80,443. Future VPC policy не включена.
- Соединяться только с проверенным literal destination, не делать повторный resolve.
- Dial с context и timeout. Не возвращать 200 до успешного dial.
- Ошибки policy → 403 generic, неверный request → 400/405, недоступный origin → 502 generic, лимит → 503. Не возвращать внутренние адреса/ключи/стек actor.
- Никогда не использовать HTTP_PROXY из ENV через http.DefaultTransport. Прямой net.Dialer до origin.

## Туннелирование

HTTP/1.1 нужен для надёжного http.Hijacker; явно ограничь ALPN и не включай auto HTTP/2 в server.
После Dial получить Hijacker, сохранить возвращённый *bufio.ReadWriter, отправить `HTTP/1.1 200 Connection Established\r\n\r\n`, Flush и копировать в обе стороны.
**Клиент→origin копировать из rw.Reader, а не только из net.Conn**: там уже могут находиться прочитанные байты tunneled TLS. Соответствующий тест обязателен.
io.Copy завершение в одну сторону не должно мгновенно обрезать уже возвращаемый ответ. Реализуй аккуратный half-close для TCP origin и bounded shutdown для TLS client. Гарантированно закрыть обе стороны при cancel/idle/max duration/error.
ReadHeaderTimeout защищает initial HTTP headers. ReadTimeout/WriteTimeout не должны случайно обрывать весь stream через несколько секунд. После hijack deadlines нужно явно пересмотреть/очистить. Idle timeout должен учитывать активность **в обеих** сторонах, а не только одну read direction. Max duration независимо ограничивает tunnel.
Ограничить одновременно активные tunnels и общий resource budget. Не создавать бесконечные goroutines при висящем origin.

## Lifecycle и observability

На SIGTERM сначала прекратить readiness/новые CONNECT, затем ограниченно drain/закрыть active hijacked connections. `http.Server.Shutdown` сам не закрывает hijacked conns: вести их registry и закрывать явно.
Структурированные connect_open/connect_close/connect_denied: proxy_id, binding id, peer URI SAN, destination, bytes in/out, duration, reason. Не логировать HTTP payload и secrets.
Нет unauthenticated proxy listener на Pod IP; TLS 8443 только mTLS.

## Обязательные tests

С сертификатами от in-memory test CA: valid/no client/unknown CA/wrong URI under trusted CA/expired/wrong EKU; origin bytes bidirectional; buffered early payload; origin error before 200; concurrency budget; cancel/idle/deadline; graceful close active tunnel.
Локальный test origin может быть loopback **только через внедряемый test Dial/Policy dependency** в библиотеке tests. Production CLI не должен иметь `--allow-private` или env bypass. Все TLS клиенты проверяют сертификаты.

Результат: compilable binary, unit/integration tests и multi-stage non-root Dockerfile с pinned base/toolchain. Нет TODO вместо streaming logic.
