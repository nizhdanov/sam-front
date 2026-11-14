# SAM Platform — backend

Бэкенд системы учёта программного обеспечения: инвентаризация компьютеров, нормализация ПО (Smart Matching),
учёт лицензий, автоматический комплаенс и AI-ассистент с ролевой моделью доступа.

## Состав

| Компонент | Порт (внутри сети) | Назначение |
| --- | --- | --- |
| nginx | 8080 (публичный) | API Gateway: маршрутизация, лимиты тела запроса, rate limiting, CORS, request id |
| processor | 8081 | Приём CSV и отчётов агента, группировка по ПК, публикация задач в NATS |
| worker | 8082 (health) | Обработка снимков из JetStream, Smart Matching, синхронизация с License Service |
| license | 8083 / 9093 gRPC | Каталог ПО, устройства, сотрудники, лицензии, EULA, Compliance Engine, аналитика |
| chat | 8084 | Диалоги пользователя, SSE-стриминг ответов |
| llm | 8085 / 9095 gRPC | AI-оркестратор: GigaChat / OpenAI-compatible / mock, ролевые инструменты |
| postgres | 5432 | `inventory_db`, `license_db`, `chat_db` с отдельными пользователями |
| nats | 4222 | JetStream, поток `INVENTORY` |

Подробности — в [docs/architecture.md](docs/architecture.md), принятые решения и ограничения — в
[docs/decisions.md](docs/decisions.md), контракт REST API — в [api/openapi.yaml](api/openapi.yaml).

## Быстрый старт

Требуется Docker с Compose v2 и Go 1.26 (для генерации ключей и демо-токенов).

```bash
make up        # создаёт .env, RSA-ключи в ./secrets, собирает и запускает стек
make seed      # демо-данные: сотрудники, устройства, каталог, лицензии
make smoke     # загрузка testdata/inventory.csv и проверка основного сценария
make tokens    # JWT для демо-пользователей employee, admin, director
```

Если Docker не может выделить подсеть, измените `DOCKER_SUBNET` в `.env`.

## Аутентификация

Токены выдаёт внешний Identity Provider, в стек он не входит. Сервисы проверяют JWT (RS256) публичным ключом
`secrets/jwt_public.pem`: issuer `sam-auth`, audience `sam-backend`, claims `sub`, `role`, `employee_id`.
Для разработки `make tokens` подписывает токены для трёх захардкоженных пользователей приватным ключом из `./secrets`.

| Пользователь | Роль | Сотрудник |
| --- | --- | --- |
| employee | EMPLOYEE | Иванов Иван (PC-001) |
| admin | ADMIN | Петрова Анна |
| director | IT_DIRECTOR | Сидоров Олег |

Межсервисные вызовы подписываются HS256-токенами с сервисными ролями (`JWT_SERVICE_SECRET`).

## Демо-сценарий

```bash
TOKEN=$(make -s tokens | sed -n '5p')                       # admin
curl -H "Authorization: Bearer $TOKEN" -F file=@testdata/inventory.csv localhost:8080/api/v1/imports
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/imports/<import_id>
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/analytics/dashboard
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/compliance/findings
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/licenses/balance
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/matching/pending

CONV=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/chat/conversations | jq -r .id)
curl -N -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"content":"Где у нас дефицит лицензий?"}' localhost:8080/api/v1/chat/conversations/$CONV/messages
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
