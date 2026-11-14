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

`generate-api` использует установленный apicraft и существующий `apicraft.config.ts`. Перегенерировать API нужно при изменении контракта. Vite и CLI маршрутов читают путь `generated/routeTree.gen.ts` из `tsr.config.json`; generated вручную не редактируется.

shadcn настроен на `src/routes/styles.css`, Base UI/base-luma и импорт `cn` из установленного пакета. Проверка без записи компонентов: `pnpm exec shadcn add @shadcn/button --dry-run`.

## API transport

Единственный Axios instance — `src/utils/lib/instance.ts`, baseURL — `/api/v1`. Vite dev/preview и контейнерный nginx проксируют этот путь на `https://sam.v1sdev.ru`, сохраняя путь. Nginx проверяет TLS upstream; лимит 51 МиБ оставляет место для multipart вокруг CSV до 50 МБ, точный лимит файла проверяет backend. Для другого размещения необходимо сохранить такой же reverse proxy до SPA fallback. Origin backend взят из URL OpenAPI в плане; production deployment ещё требует сетевой проверки у владельца стенда.

Токен читается из `localStorage.token` перед каждым запросом. Пустое значение не создаёт Authorization, недоступный localStorage блокирует отправку. Запись токенов и переключатель ролей относятся к этапу 2.

Generated-функции возвращают AxiosResponse; данные находятся в `.data`. В `config.signal` передаётся AbortSignal. HTTP/сетевые ошибки отклоняют Promise как `ApiError` с `status`, безопасным `message`, `code` и `requestId`. Отмена сохраняется как Axios cancellation (`isCancel`); UI-обработчик 401 подключается на этапе 2. Автоматического повтора мутаций нет.

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
