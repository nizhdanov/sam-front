# SAM Platform — backend

Бэкенд системы учёта программного обеспечения: инвентаризация компьютеров, нормализация ПО (Smart Matching),
учёт лицензий, автоматический комплаенс и AI-ассистент с ролевой моделью доступа.

## Состав

| Компонент | Порт (внутри сети) | Назначение |
| --- | --- | --- |
| nginx | 8080 (публичный) | API Gateway: маршрутизация, лимиты тела запроса, rate limiting, CORS, request id |
| processor | 8081 | Приём CSV и отчётов агента, группировка по ПК, публикация задач в NATS |
| worker | 8082 (health) | Обработка снимков из JetStream, Smart Matching, синхронизация с License Service |
| license | 8083 / 9093 gRPC | Каталог ПО, устройства, договоры, лицензии, EULA, Compliance Engine, аналитика |
| chat | 8084 | Диалоги пользователя, SSE-стриминг ответов |
| llm | 8085 / 9095 gRPC | AI-оркестратор: GigaChat / OpenAI-compatible / mock, ролевые инструменты |
| postgres | 5432 | `inventory_db`, `license_db`, `chat_db` с отдельными пользователями |
| nats | 4222 | JetStream, поток `INVENTORY` |

Подробности — в [docs/architecture.md](docs/architecture.md), принятые решения и ограничения — в
[docs/decisions.md](docs/decisions.md), контракт REST API — в [api/openapi.yaml](api/openapi.yaml).

Swagger UI: `/api/v1/docs` (на сервере — https://sam.v1sdev.ru/api/v1/docs).
В `Authorize` вставьте JWT без префикса `Bearer`, затем используйте `Try it out`.
Запросы отправляются на текущий домен. Документация открыта без авторизации;
API сохраняет проверку JWT и ролей. Swagger UI 5.35.0 размещён локально в `api/docs/vendor`.

## Быстрый старт

Требуется Docker с Compose v2 и Go 1.26 (для генерации ключей и демо-токенов).

```bash
make up        # создаёт .env, RSA-ключи в ./secrets, собирает и запускает стек
make seed      # демо-данные: устройства, каталог, соглашения, договоры, лицензии
make smoke     # загрузка testdata/inventory.csv и проверка основного сценария
make tokens    # JWT для демо-пользователей user и admin
```

Если Docker не может выделить подсеть, измените `DOCKER_SUBNET` в `.env`.

## Деплой за серверным nginx

Для `sam.v1sdev.ru` используется `compose.server.yaml` вместе с основным compose:

```bash
docker compose -f compose.yaml -f compose.server.yaml up -d --build
```

В `.env` нужны `DOMAIN=sam.v1sdev.ru`, уникальные пароли БД и `JWT_SERVICE_SECRET`.
JWT-ключи находятся в `secrets/`. Существующие сертификаты сервера монтируются из
`/etc/letsencrypt`; отдельный certbot в этом варианте не запускается.

Шлюз бэкенда слушает только `127.0.0.1:8443` (HTTPS). В существующий HTTPS server
основного nginx добавляется содержимое `deploy/nginx/snippets/host-backend.conf`:
все `/api/` идут в шлюз, где сохраняется маршрутизация processor/chat/license.
Существующий `location /` остаётся направленным на фронтенд `127.0.0.1:5173`.
Порты 80, 443 и занятый Apache порт 8080 контейнеры не занимают.

После обновления сертификата нужно перечитать его и в контейнере:

```bash
docker compose -f compose.yaml -f compose.server.yaml exec -T nginx nginx -s reload
```

На сервере это выполняет deploy-hook certbot
`/etc/letsencrypt/renewal-hooks/deploy/50-sam-backend.sh`
(исходник: `deploy/reload-backend-certificate.sh`).

На сервере проект размещается в `/opt/sam-backend`. Демо-данные автоматически
не загружаются. По умолчанию AI использует `mock`.

## Аутентификация

Токены выдаёт внешний Identity Provider, в стек он не входит. Сервисы проверяют JWT (RS256) публичным ключом
`secrets/jwt_public.pem`: issuer `sam-auth`, audience `sam-backend`, claims `sub`, `role`, `device_serial`.
Для разработки `make tokens` подписывает токены для двух захардкоженных пользователей приватным ключом из `./secrets`.

| Пользователь | Роль | Устройство |
| --- | --- | --- |
| user | USER | PC-001 |
| admin | ADMIN | — |

USER видит справочник БПО, бесплатное и запрещённое ПО, свой компьютер (из `device_serial`) и задаёт чату вопросы
только по бесплатному ПО. ADMIN — администратор SAM с полным доступом.

Межсервисные вызовы подписываются HS256-токенами с сервисными ролями (`JWT_SERVICE_SECRET`).

## Демо-сценарий

```bash
TOKEN=$(make -s tokens | sed -n '5p')                       # admin
curl -H "Authorization: Bearer $TOKEN" -F file=@testdata/inventory.csv localhost:8080/api/v1/imports   # предпросмотр
curl -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/imports/<import_id>/confirm
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/imports/<import_id>                    # отчёт
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/dashboard
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/balance
curl -H "Authorization: Bearer $TOKEN" "localhost:8080/api/v1/data/installations?product=photoshop"
curl -H "Authorization: Bearer $TOKEN" "localhost:8080/api/v1/data/findings?finding_type=FORBIDDEN_SOFTWARE&format=xlsx" -o forbidden.xlsx
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/matching/pending

CONV=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/chat/conversations | jq -r .id)
curl -N -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"content":"У кого стоит нелицензионный Adobe Photoshop?"}' localhost:8080/api/v1/chat/conversations/$CONV/messages
```

Демо-данные содержат: дефицит лицензий Microsoft Office и Adobe Photoshop, истёкшую лицензию TeamViewer,
запрещённое ПО, неподдерживаемые версии, неоднозначную редакцию Visual Studio, неизвестное ПО и метрику CONCURRENT,
которую нельзя проверить по данным установок.

## GigaChat

По умолчанию `LLM_PROVIDER=mock`: ответы помечены `[DEMO]`, AI-сопоставление не выполняется, ПО уходит на ручную проверку.
Для GigaChat в `.env`:

```dotenv
LLM_PROVIDER=gigachat
GIGACHAT_AUTH_KEY=<authorization key из личного кабинета>
GIGACHAT_SCOPE=GIGACHAT_API_PERS
LLM_CHAT_MODEL=GigaChat
```

Вместо `GIGACHAT_AUTH_KEY` можно задать `GIGACHAT_CLIENT_ID` и
`GIGACHAT_CLIENT_SECRET`: сервис сам кодирует пару в Base64. Если задан готовый ключ,
он имеет приоритет. Секреты хранятся только в `.env`, не в репозитории.

Access token получается автоматически при первом запросе к GigaChat, кэшируется в памяти
и обновляется при следующем запросе, если до истечения осталось не более минуты.
При ответе 401 сервис сбрасывает отклонённый токен, получает новый и повторяет запрос один раз.
Параллельные запросы используют общий токен; пустые и просроченные ответы OAuth не кэшируются.
После рестарта токен будет получен заново, вручную копировать его не нужно.

GigaChat использует сертификат НУЦ Минцифры. Положите корневой сертификат в `secrets/russian_trusted_root_ca.pem`
и укажите `GIGACHAT_CA_CERT_PATH=/secrets/russian_trusted_root_ca.pem` (смонтировав файл в сервис `llm`),
либо для локальной демонстрации временно выставьте `GIGACHAT_INSECURE_SKIP_VERIFY=true`.
Любой OpenAI-совместимый провайдер подключается через `LLM_PROVIDER=openai_compatible`, `LLM_BASE_URL`, `LLM_API_KEY`.

## Команды

```
make up | down [V=1] | ps | logs [SERVICE=name]
make seed | smoke | tokens | keys
make build | vet | fmt | proto | lint
```

`make proto` генерирует gRPC-код через `buf` (запускается через `go run`, локальная установка не нужна).
