# SAM frontend

Vite SPA на React и TypeScript. План реализации и границы этапов — в [PLAN.md](./PLAN.md).

## Разработка и проверки

Зависимости уже описаны в pnpm lockfile. Для тестов нужен Node.js 24+ с TypeScript stripping и `module.registerHooks` (проверено на 26.10.0); отдельный тестовый фреймворк не используется.

```sh
pnpm dev
pnpm generate-routes
pnpm generate-api
pnpm typecheck
pnpm test
pnpm build
pnpm exec eslint src/utils/api src/utils/lib/instance.ts tests vite.config.ts eslint.config.js
```

`generate-api` использует установленный apicraft и существующий `apicraft.config.ts`, затем нормализует разделители путей в импортах через `scripts/normalize-api-imports.mjs`. Это сохраняет совместимость с Linux/Docker после генерации на Windows. Для нормализации уже созданного клиента можно отдельно запустить `node scripts/normalize-api-imports.mjs`. Перегенерировать API нужно при изменении контракта. Vite и CLI маршрутов читают путь `generated/routeTree.gen.ts` из `tsr.config.json`; generated вручную не редактируется.

shadcn настроен на `src/routes/styles.css`, Base UI/base-luma и импорт `cn` из установленного пакета. Проверка без записи компонентов: `pnpm exec shadcn add @shadcn/button --dry-run`.

## API transport

Единственный Axios instance — `src/utils/lib/instance.ts`, baseURL — `/api/v1`. Vite dev/preview и контейнерный nginx проксируют этот путь на `https://sam.v1sdev.ru`, сохраняя путь. Nginx проверяет TLS upstream; лимит 51 МиБ оставляет место для multipart вокруг CSV до 50 МБ, точный лимит файла проверяет backend. Для другого размещения необходимо сохранить такой же reverse proxy до SPA fallback. Origin backend взят из URL OpenAPI в плане; production deployment ещё требует сетевой проверки у владельца стенда.

Токен читается из `localStorage.token` перед каждым запросом. Пустое значение не создаёт Authorization, недоступный localStorage блокирует отправку. Переключатель «Роль» использует существующие демонстрационные значения из `src/utils/constants/tokens.ts` через таблицу `src/app/auth/demo-accounts.ts`. Первый запуск/неизвестный токен выбирает USER; известный ADMIN сохраняется после reload. Значения предназначены для MVP; истёкшие токены заменяются в исходной таблице, автоматического refresh нет.

Generated-функции возвращают AxiosResponse; данные находятся в `.data`. В `config.signal` передаётся AbortSignal. HTTP/сетевые ошибки отклоняют Promise как `ApiError` с `status`, безопасным `message`, `code` и `requestId`. Отмена сохраняется как Axios cancellation (`isCancel`). Актуальный 401 очищает защищённые данные и оставляет доступным Select для явного повторного выбора; поздний 401 старого контекста игнорируется. Автоматического повтора мутаций нет; GET повторяется не более двух раз при сети/5xx, без повторов 4xx.

## Оболочка, роли и общие таблицы (этап 2)

`src/app/auth/session.ts` управляет текущей ролью и номером контекста. Смена роли сначала закрывает защищённое содержимое, отменяет Query/Axios-запросы и очищает Query/Mutation cache, затем записывает токен и открывает landing: ADMIN → `/dashboard`, USER → `/software`. Событие `storage` синхронизирует другие вкладки без повторной записи; удалённый/неизвестный токен другой вкладки блокирует данные до явного выбора. В localStorage хранится только `token`.

Вложенные `_authenticated` и `_admin` выполняют guards до loaders/preload. Их предметные страницы пока показывают «Раздел готовится»; это маршруты оболочки, реализация данных начинается с этапа 3. Меню USER содержит каталог и «Мой компьютер». Права на объекты и фактическую проекцию ответа проверяет backend.

Новые страницы получают `session` из route context. Ключ Query должен включать `session.role`, `session.epoch`, ресурс и все фильтры; для запроса используется `auth.requestConfig(session)` и Query AbortSignal (при необходимости через `AbortSignal.any`). Перед изменением UI в асинхронном callback проверяется `auth.isCurrent(session.epoch)`. Фоновая работа, включая будущий SSE, должна использовать `session.signal`; уже принятый сервером POST отмена браузера не откатывает.

`src/components/shared/states.tsx` предоставляет loading/empty/error/success. `DataTable` из `src/components/shared/data-table.tsx` принимает `dataset`, исходный ответ `data`, `caption`, текущие `filters`, необязательные `onResetFilters` и `renderCell(value, column)` для предметных подписей. Компонент связывает значения по имени колонки, проверяет длины строк/счётчики, скрывает административные и неизвестные колонки и показывает `shown/total`. При пустом фильтре и ошибке используются разные состояния; скролл остаётся внутри таблицы. `FormLeaveGuard` защищает формы CSV и устройства через Router blocker/Sheet и beforeunload.

`src/utils/api/data.ts` содержит `fetchDataTable`, `getDataParams`, `projectDataTable` и `exportDataTable`. Whitelist фильтров и группировок соответствует описанию `/data/{dataset}`, включая параметры, отсутствующие в generated query. `recognized=false` и `in_bpo=false` сохраняются. Весь нормализованный набор передаётся через `config.params`, поэтому экспорт CSV/XLSX использует те же фильтры и группировку. По умолчанию чтение ограничено 100 строками, экспорт — 10 000; явно заданный `limit` сохраняется. Пагинация для `/data/*` контрактом не предусмотрена (B-03).

Проверки этапа 2 и полный список изменённых файлов записаны в [PLAN.md](./PLAN.md). Дополнительно к API self-check `pnpm test` проверяет permissions, lifecycle ролей, поздние ответы/401/submit, localStorage и таблицы/экспорт. Браузерные проверки выполнялись в Edge без установки нового тестового раннера.

Доступная часть этапа 3: `/software` и `/software/$productId` используют generated-клиент и Query для обеих ролей; `/contracts` — реестр ADMIN через `/data/contracts`. Фильтры находятся в URL, каталог имеет серверную пагинацию, договоры показывают `shown/total` и экспортируют текущие фильтры/limit. Служебные поля продукта удаляются до кеширования USER; это дополнительная клиентская проекция, серверные права проверяются отдельно. Предметный код находится в `src/features/software` и `src/features/contracts`, маршруты остаются тонкими.

Типизированная часть TASK-007 находится в `src/features/agreements`: ADMIN создаёт соглашение с `/agreements`, после сохранения открывается `/agreements/$agreementId`. Карточка восстанавливается по прямой ссылке, поддерживает редактирование, multipart-загрузку .txt/.pdf/.docx и подтверждённое удаление. `commercial_allowed` сохраняет true/false/null; пустые текст и источник очищаются через null. Форма сохраняет черновик при ошибке и защищает от случайного ухода. Консервативный лимит файла — 10 000 000 байт до уточнения серверной границы 10 МБ. Изменения отменяются на клиенте при смене роли; уже принятый сервером запрос это не откатывает.

Этап 3 пока не завершён: согласованные typed ответы списка соглашений, версий и карточки договора отсутствуют (B-02a/B-02b). Список соглашений явно обозначен недоступным, создание не имитирует его локальными записями. CRUD продуктов/договоров, версии и ссылка из реестра в карточку договора не добавлены. Новые операции соглашений проверены локальным HTTP fixture и в браузере, серверная CRUD-приёмка ещё не выполнена. После прежнего явного разрешения пользователя выполнены read-only проверки стенда: каталог обеих ролей, серверная проекция/скрытый UUID USER, запрет договоров и их экспорта для USER, реестр и CSV/XLSX ADMIN. TASK-008 завершена; остальные части B-04 проверяются на своих этапах. Входной адаптер допускает null в незаполненных текстовых полях продукта, фактически возвращаемый backend вопреки optional string в OpenAPI. Подробности и порядок продолжения — в [PLAN.md](./PLAN.md).

Доступная часть этапа 4: `/imports` и `/imports/$importId` загружают CSV, восстанавливают серверный preview, подтверждают/отменяют импорт, показывают прогресс, полный отчёт и историю с пагинацией. CSV не разбирается в браузере; `collected_at` отправляется с часовым поясом, консервативная граница файла — 50 000 000 байт. Обработка не запускается автоматически. Retry доступен только для неуспешных снимков; ошибки строк не выдаются за снимки для повторной обработки. Polling работает только для активной карточки и останавливается при скрытии вкладки, уходе, смене роли и терминальном статусе.

`/devices` — ADMIN-реестр с фильтрами и `shown/total`. `/devices/$deviceId` показывает карточку, отдельные таблицы установок/нарушений и историю снимков; ADMIN редактирует только тип, NEW→ACTIVE, hostname/responsible/location. USER получает проекцию без служебной истории и ручных полей, доступ к чужому устройству ограничивает backend. `inventory_id` не используется как `import_id`. `/installations` поддерживает документированные фильтры, `recognized=false`, группы по продукту/серийному номеру и переход к исходным записям; групповые количества устройств не складываются в размер парка. Текстовые фильтры API ищут по подстроке, это относится и к фильтру серийного номера в карточке.

Код этапа 4 использует существующие generated-функции, Query и DataTable. Реальные GET/CSV/XLSX стенда проверены; полная приёмка этапа остаётся открытой: подробные ошибки импорта требуют B-02a, полный обход усечённых данных — B-03, своего устройства USER на стенде нет. Серверные CSV/PATCH проверки отклонены автоматической проверкой из-за постоянных тестовых записей без удаления; заказчик явно решил оставить серверную приёмку незавершённой. Эти сценарии проверены только на локальном HTTP/browser fixture. Контрольные результаты и полный список изменённых файлов — в протоколе этапа 4 [PLAN.md](./PLAN.md). Этап 5 не начинался.

Загрузки CSV и EULA используют `multipartConfig` через `config.data`, без ручного Content-Type или boundary:

```ts
import { postImports } from '@/generated/api';
import { multipartConfig } from '@/utils/api';

const body = { file };
const response = await postImports({
  body,
  query: { confirm: false },
  config: { ...multipartConfig(body), signal }
});
```

Для скачивания у generated-запроса задаётся `responseType: 'blob'`, затем весь ответ передаётся `downloadFile`. JSON-ошибка, включая ответ с HTTP 200, не сохраняется как файл; имя берётся из Content-Disposition с безопасным fallback, object URL освобождается.

```ts
import { getDataByDataset } from '@/generated/api';
import { downloadFile } from '@/utils/api';

const response = await getDataByDataset({
  path: { dataset: 'devices' },
  query: { format: 'csv' },
  config: { responseType: 'blob', signal }
});
await downloadFile(response, 'devices.csv');
```

`pnpm test` проверяет нормализацию ошибок, имена и очистку скачивания, токен и недоступный localStorage. Локальный HTTP self-check вызывает настоящий generated-клиент: multipart для обоих upload endpoints, JSON и бинарный экспорт, JSON-ошибки в Blob и отмену запросов. Для multipart используется Node HTTP adapter, для Blob — fetch adapter; браузер использует штатный Axios adapter.
