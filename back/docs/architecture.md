# Архитектура

## Схема

```
                         ┌──────────────────────── nginx :8080 ────────────────────────┐
  клиент / агент ──────▶ │ /api/v1/imports, /agent/inventory → processor              │
                         │ /api/v1/chat/*                    → chat (SSE, без буфера)  │
                         │ /api/v1/*                         → license                 │
                         └─────────────────────────────────────────────────────────────┘
                                │                   │                     │
                                ▼                   ▼                     ▼
                         ┌────────────┐      ┌────────────┐  gRPC   ┌────────────┐
                         │ processor  │      │    chat    │ ──────▶ │    llm     │
                         └─────┬──────┘      └─────┬──────┘ Chat    └──┬──────▲──┘
                   inventory_db│  publish          │chat_db   RunQuery │      │ AnalyzeAgreement
                               ▼                   ▼                   ▼      │
                         ┌──────────┐                            ┌────────────┴┐
                         │ NATS JS  │                            │   license   │── license_db
                         │INVENTORY │                            └──────▲──────┘
                         └────┬─────┘                                   │ gRPC: ResolveSoftwareBatch,
                              ▼                                         │ SaveMatchingSuggestion,
                         ┌──────────┐ ── gRPC MatchSoftware ──▶ llm     │ SyncInventory
                         │  worker  │ ──────────────────────────────────┘
                         └──────────┘── inventory_db (статусы снимков)
```

Внешний трафик проходит только через nginx. Сервисы публикуют собственные REST-ручки под `/api/v1`, nginx лишь
маршрутизирует, ограничивает размер тела и частоту запросов, добавляет `X-Request-Id` и CORS. Межсервисное
взаимодействие — gRPC (`proto/license/v1`, `proto/llm/v1`) и NATS JetStream.

## Владение данными

| БД | Владелец (миграции) | Кто ещё пишет | Таблицы |
| --- | --- | --- | --- |
| `inventory_db` | processor | worker (статусы, lease) | imports, inventory_snapshots, import_errors |
| `license_db` | license | — | employees, devices, software_products/versions/aliases, installations, license_agreements, license_entitlements, matching_candidates, processed_snapshots, compliance_findings, agreement_analyses |
| `chat_db` | chat | — | conversations, messages |

Каждый сервис применяет свои миграции goose при старте (с advisory-локом), подключается отдельным пользователем
БД. Сервис llm без собственного хранилища.

## Поток инвентаризации

1. **Приём.** `POST /api/v1/imports` (multipart, до 50 МБ) — файл потоково пишется в
   `/data/imports/<id>/inventory.csv.tmp`, затем `rename`; создаётся запись `imports` в статусе `PENDING`, ответ `202`.
   Агент шлёт JSON на `POST /api/v1/agent/inventory` — это импорт из одного снимка.
2. **Разбор.** Ограниченный пул парсеров: BOM, автоопределение `,`/`;`, проверка заголовков, нормализация полей,
   дедупликация строк, ошибки строк в `import_errors`. Строки группируются по `serial_number` в снимки
   (`inventory_snapshots`). Критическая ошибка формата → импорт `FAILED` без публикации. Незавершённые
   `PENDING/PARSING` импорты подхватываются после рестарта.
3. **Публикация.** Для каждого снимка — событие `inventory.snapshot.created.v1` в поток `INVENTORY`
   (WorkQueue, file storage) с `Nats-Msg-Id = snapshot_id` для дедупликации; повтор с экспоненциальной задержкой.
4. **Обработка.** worker — durable pull consumer (`AckWait 120s`, `MaxDeliver 5`, `MaxAckPending 8`, 2 воркера).
   Снимок захватывается `UPDATE … RETURNING attempt_token` с lease; во время работы шлётся `InProgress` и
   продлевается lease. Завершение записывается только с совпадающим `attempt_token`.
5. **Сопоставление.** Уникальные `source_key` (sha256 нормализованных имени/издателя/версии) ищутся в LRU-кэше,
   промахи — батчами по 500 через `ResolveSoftwareBatch`. Ответ license: `MATCHED`, `AMBIGUOUS` (кандидаты из
   pg_trgm), `UNKNOWN`, `PENDING_REVIEW`. Неоднозначные отправляются в `llm.MatchSoftware`; ответ с уверенностью
   выше порога сохраняется как alias, иначе — задача ручной проверки. Ошибка LLM не валит снимок: ПО уходит в
   `REQUIRES_REVIEW`.
6. **Синхронизация.** `SyncInventory` в одной транзакции: идемпотентность по `processed_snapshots`, отсев
   устаревших снимков по `collected_at` (`SUPERSEDED`), upsert устройства и установок, деактивация отсутствующих
   при `FULL`-снимке, advisory-локи по затронутым продуктам, инкрементальный пересчёт комплаенса.
7. **Статус импорта** вычисляется из агрегатов снимков (`PROCESSING` → `COMPLETED` / `COMPLETED_WITH_ERRORS` /
   `FAILED`).

Временные ошибки → `Nak` с задержкой; постоянные или исчерпанные попытки → `Term` и снимок `FAILED`.

## Compliance Engine

Детерминированные правила в `services/license/internal/domain/compliance.go`, LLM в расчёте не участвует.

| Тип | Условие |
| --- | --- |
| `FORBIDDEN_SOFTWARE` | установлен продукт с политикой `FORBIDDEN` |
| `LICENSE_DEFICIT` | потребление по `PER_DEVICE`/`PER_USER` больше суммы действующих прав |
| `LICENSE_EXPIRED` | коммерческий продукт установлен, все права истекли |
| `LICENSE_UNVERIFIED` | коммерческий продукт без прав или с метрикой, которую нельзя посчитать (`CONCURRENT`, `UNKNOWN`) |
| `UNSUPPORTED_SOFTWARE` / `OUTDATED_SOFTWARE` | версия с lifecycle `UNSUPPORTED` / `OUTDATED` |
| `UNKNOWN_SOFTWARE` | ПО не сопоставлено с каталогом |
| `COMMERCIAL_USE_RESTRICTED` | соглашение запрещает коммерческое использование |

Находки адресуются стабильным `finding_key`: повторный расчёт обновляет существующую запись, исчезнувшее
нарушение переводится в `RESOLVED`, подтверждённое администратором остаётся `ACKNOWLEDGED`. Пересчёт запускается
при синхронизации инвентаря, изменении каталога/лицензий/соглашений, вручную и периодически (истечение сроков).

## Безопасность и RBAC

- JWT RS256 от внешнего IdP (`iss=sam-auth`, `aud=sam-backend`) проверяется **каждым** сервисом; заголовкам с
  ролью сервисы не доверяют.
- Межсервисные вызовы: HS256-токены с сервисными ролями (`INVENTORY_WORKER`, `CHAT_SERVICE`, `LLM_SERVICE`, …).
  Когда вызов совершается от имени пользователя (chat → llm → license), пробрасывается исходный токен пользователя,
  и license применяет его роль.
- Ролевые проверки выполняются в сервисном слое license, а не в промптах:

| Роль | Доступ |
| --- | --- |
| `EMPLOYEE` | свои устройства и ПО, каталог без цен, свои находки, урезанная сводка; без лицензий, цен, соглашений |
| `ADMIN` | всё, включая изменение каталога, лицензий, сопоставлений, подтверждение находок |
| `IT_DIRECTOR` | чтение всего, включая стоимость и аналитику; без изменений |

## AI-ассистент

- chat хранит диалоги (владелец + роль на момент создания), отдаёт ответ через SSE
  (`message_start`, `delta`, `message_end`, `error`), один активный запрос на пользователя, контекст — 12 сообщений.
- llm собирает системный промпт по роли и набор инструментов из allowlist роли. Каждый инструмент — это вызов
  `AssistantDataService.RunQuery` в license с токеном пользователя, поэтому даже при обходе промпта данные режутся
  RBAC license. Результат инструмента ограничен 6000 байт, цикл вызовов — 5 итераций.
- Провайдеры: `gigachat` (OAuth, кеш токена, `functions`), `openai_compatible`, `mock` (детерминированный, `[DEMO]`).
- Ограничения: семафор параллельных запросов, таймауты, дневной бюджет токенов.

## Наблюдаемость

- JSON-логи `slog` с `request_id`, сквозной `x-request-id` в gRPC metadata.
- `/healthz` (liveness) и `/readyz` (БД, NATS, зависимые gRPC) у каждого сервиса; Docker `HEALTHCHECK` вызывает
  подкоманду `/app healthcheck` того же бинарника (distroless-образ без shell и curl).
