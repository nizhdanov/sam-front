import { AxiosError, CanceledError, isCancel } from 'axios';
import assert from 'node:assert/strict';
import { Buffer } from 'node:buffer';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { registerHooks } from 'node:module';
import { after, beforeEach, test } from 'node:test';

// Node запускает исходные TS и generated-импорты с алиасами и Windows-разделителями.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('@/')) {
      const root = specifier.startsWith('@/generated/') ? '../' : '../src/';
      return nextResolve(new URL(`${root}${specifier.slice(2)}.ts`, import.meta.url).href, context);
    }
    if (specifier.startsWith('.')) {
      const path = specifier.replaceAll('\\', '/');
      return nextResolve(/\.(?:[cm]?js|tsx?)$/.test(path) ? path : `${path}.ts`, context);
    }
    return nextResolve(specifier, context);
  }
});

const { ApiError, normalizeApiError, readApiError } = await import('../src/utils/api/errors.ts');
const { downloadFile, getDownloadFilename } = await import('../src/utils/api/files.ts');
const { multipartConfig } = await import('../src/utils/api/multipart.ts');
const { instance } = await import('../src/utils/lib/instance.ts');
const { postImports } = await import('../generated/api/requests/imports/postImports.gen.ts');
const { postAgreementByIdFile } =
  await import('../generated/api/requests/agreements/postAgreementByIdFile.gen.ts');
const { getDataByDataset } = await import('../generated/api/requests/data/getDataByDataset.gen.ts');
const { postExport } = await import('../generated/api/requests/data/postExport.gen.ts');
const { getSoftware } = await import('../generated/api/requests/software/getSoftware.gen.ts');

const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
let token = null;
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: { getItem: (key) => (key === 'token' ? token : null) }
});
beforeEach(() => {
  token = null;
});
after(() => {
  if (storageDescriptor) Object.defineProperty(globalThis, 'localStorage', storageDescriptor);
  else delete globalThis.localStorage;
});

test('ошибки сохраняют HTTP-статус и только безопасные поля контракта', async () => {
  const error = await readApiError(
    {
      error: {
        code: 'CONFLICT',
        message: 'Объект используется.',
        request_id: 'request-1',
        stack: 'private'
      }
    },
    409
  );
  assert.ok(error instanceof ApiError);
  assert.equal(error.status, 409);
  assert.equal(error.code, 'CONFLICT');
  assert.equal(error.message, 'Объект используется.');
  assert.equal(error.requestId, 'request-1');
  assert.equal(error.stack.includes('private'), false);
  assert.equal(await normalizeApiError(error), error);
});

test('некорректные ответы, HTML, JWT и stack не попадают в сообщение', async () => {
  for (const payload of [
    null,
    '<html>proxy error</html>',
    { error: { message: 42 } },
    { error: { message: 'Bearer secret' } },
    { error: { message: 'eyJhbGci.aBc.sIg' } },
    { error: { message: 'Error\n    at privateFunction' } }
  ]) {
    const error = await readApiError(payload, 403);
    assert.equal(error.message, 'Доступ запрещён.');
  }
  for (const status of [400, 401, 404, 409, 413, 429, 500, 503]) {
    const error = await readApiError(undefined, status);
    assert.equal(error.status, status);
    assert.ok(error.message.length > 0);
  }
  const network = await normalizeApiError(new AxiosError('private URL and headers', 'ERR_NETWORK'));
  assert.equal(network.status, undefined);
  assert.match(network.message, /подключение/);
  const cancelled = new CanceledError();
  assert.equal(await normalizeApiError(cancelled), cancelled);
});

test('имя файла: UTF-8, кавычки, повреждённое кодирование и опасные пути', () => {
  const cases = [
    [
      "attachment; filename=old.csv; filename*=UTF-8'ru'%D0%BE%D1%82%D1%87%D1%91%D1%82.csv",
      'отчёт.csv'
    ],
    ['attachment; filename="report; final.csv"', 'report; final.csv'],
    ["attachment; filename=report.csv; filename*=UTF-8''%ZZ", 'report.csv'],
    ['attachment; filename="../../report.csv"', 'report.csv'],
    ['attachment; filename="C:\\temp\\report.csv"', 'report.csv'],
    ['attachment; filename="CON.csv"', '_CON.csv'],
    ['attachment; filename="a\u202Eb?.csv"', 'a_b_.csv'],
    ['attachment; filename=".."', 'fallback.csv'],
    [undefined, 'fallback.csv']
  ];
  for (const [header, expected] of cases)
    assert.equal(getDownloadFilename(header, 'fallback.csv'), expected);
  assert.equal(getDownloadFilename(null, '../..'), 'download');
});

test('interceptor читает текущий token и не отправляет пустой Authorization', async () => {
  for (const value of [null, '', '  ', 'first-demo', 'second-demo']) {
    token = value;
    const response = await instance.get('/test', {
      headers: { Authorization: 'stale' },
      adapter: async (config) => ({
        data: null,
        status: 200,
        statusText: 'OK',
        headers: {},
        config
      })
    });
    assert.equal(
      response.config.headers.get('Authorization'),
      value?.trim() ? `Bearer ${value}` : undefined
    );
  }
});

test('недоступный localStorage блокирует запрос до отправки', async (t) => {
  t.mock.method(localStorage, 'getItem', () => {
    throw new Error('storage denied');
  });
  let sent = false;
  await assert.rejects(
    instance.get('/test', {
      adapter: async () => {
        sent = true;
        throw new Error('must not send');
      }
    }),
    (error) => error instanceof ApiError && /localStorage/.test(error.message)
  );
  assert.equal(sent, false);
});

test('скачивание освобождает object URL и удаляет временную ссылку', async (t) => {
  const events = [];
  const link = { click: () => events.push('click'), remove: () => events.push('remove') };
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: { createElement: () => link, body: { append: () => events.push('append') } }
  });
  t.after(() => {
    delete globalThis.document;
  });
  t.mock.method(URL, 'createObjectURL', () => 'blob:test');
  t.mock.method(URL, 'revokeObjectURL', (url) => events.push(url));
  t.mock.timers.enable({ apis: ['setTimeout'] });
  await downloadFile(
    { data: new Blob(['a,b']), status: 200, headers: { 'content-type': 'text/csv' } },
    'report.csv'
  );
  assert.equal(link.download, 'report.csv');
  assert.equal(link.href, 'blob:test');
  assert.deepEqual(events, ['append', 'click', 'remove']);
  t.mock.timers.tick(1000);
  assert.deepEqual(events, ['append', 'click', 'remove', 'blob:test']);
  await assert.rejects(
    downloadFile({ data: {}, status: 200, headers: {} }, 'report.csv'),
    ApiError
  );
});

test('HTTP self-check generated: multipart, JSON, blob-ошибки и AbortSignal', async (t) => {
  const uploads = [];
  let exported;
  let started;
  const server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url, 'http://localhost');
      if (url.pathname === '/software') {
        started();
        return;
      }
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = Buffer.concat(chunks);
      if (url.pathname === '/imports' || url.pathname === '/agreements/demo/file') {
        const form = await new Request('http://localhost', {
          method: 'POST',
          headers: request.headers,
          body
        }).formData();
        const file = form.get('file');
        uploads.push({
          path: url.pathname,
          query: url.search,
          name: file.name,
          text: await file.text(),
          authorization: request.headers.authorization,
          contentType: request.headers['content-type']
        });
        response.writeHead(url.pathname === '/imports' ? 202 : 200, {
          'content-type': 'application/json'
        });
        response.end(JSON.stringify({ import_id: 'demo', status: 'AWAITING_CONFIRMATION' }));
        return;
      }
      if (url.pathname === '/export') {
        exported = JSON.parse(body.toString());
        response.writeHead(200, {
          'content-type': 'text/csv',
          'content-disposition': 'attachment; filename="report.csv"'
        });
        response.end('product,count\nSAM,1');
        return;
      }
      response.writeHead(url.searchParams.get('format') === 'xlsx' ? 200 : 403, {
        'content-type': 'application/problem+json'
      });
      response.end(
        JSON.stringify({
          error: { code: 'FORBIDDEN', message: 'Нет доступа к выгрузке.', request_id: 'blob-1' }
        })
      );
    } catch (error) {
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ error: { message: error.message } }));
    }
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  t.after(
    () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(resolve);
      })
  );
  const config = { baseURL: `http://127.0.0.1:${server.address().port}`, adapter: 'fetch' };

  await t.test('CSV и EULA доходят до сервера как файл с multipart boundary', async () => {
    token = 'demo-admin';
    const csv = {
      file: new File(
        ['COMPUTERS.SERIAL_NUMBER;SOFTWARE.DISPLAYNAME\nVMware-42 37;SAM'],
        'inventory.csv'
      )
    };
    const result = await postImports({
      body: csv,
      query: { confirm: false },
      config: { ...config, adapter: 'http', ...multipartConfig(csv) }
    });
    assert.equal(result.status, 202);
    const eula = { file: new File(['Условия соглашения'], 'license.txt') };
    await postAgreementByIdFile({
      path: { id: 'demo' },
      body: eula,
      config: { ...config, adapter: 'http', ...multipartConfig(eula) }
    });
    assert.equal(uploads.length, 2);
    assert.equal(uploads[0].query, '?confirm=false');
    assert.equal(uploads[0].name, 'inventory.csv');
    assert.match(uploads[0].text, /VMware-42 37;SAM/);
    assert.equal(uploads[1].name, 'license.txt');
    assert.equal(uploads[1].text, 'Условия соглашения');
    for (const upload of uploads) {
      assert.match(upload.contentType, /^multipart\/form-data; boundary=/);
      assert.equal(upload.authorization, 'Bearer demo-admin');
    }
  });

  await t.test('обычное JSON-тело экспорта сохраняется, ответ является Blob', async () => {
    const body = { columns: ['product', 'count'], rows: [['SAM', 1]], format: 'csv' };
    const result = await postExport({ body, config: { ...config, responseType: 'blob' } });
    assert.deepEqual(exported, body);
    assert.ok(result.data instanceof Blob);
    assert.equal(await result.data.text(), 'product,count\nSAM,1');
  });

  await t.test('HTTP-ошибка в Blob нормализуется с message/request_id', async () => {
    await assert.rejects(
      getDataByDataset({
        path: { dataset: 'devices' },
        query: { format: 'csv' },
        config: { ...config, responseType: 'blob' }
      }),
      (error) => {
        assert.ok(error instanceof ApiError);
        assert.equal(error.status, 403);
        assert.equal(error.message, 'Нет доступа к выгрузке.');
        assert.equal(error.requestId, 'blob-1');
        return true;
      }
    );
  });

  await t.test('JSON-ошибка с HTTP 200 не скачивается как файл', async () => {
    const result = await getDataByDataset({
      path: { dataset: 'devices' },
      query: { format: 'xlsx' },
      config: { ...config, responseType: 'blob' }
    });
    await assert.rejects(
      downloadFile(result, 'report.xlsx'),
      (error) => error instanceof ApiError && error.code === 'FORBIDDEN'
    );
  });

  await t.test('отмена generated-запроса сохраняет axios.isCancel', async () => {
    const controller = new AbortController();
    const received = new Promise((resolve) => {
      started = resolve;
    });
    const pending = getSoftware({ config: { ...config, signal: controller.signal } });
    await received;
    controller.abort();
    await assert.rejects(pending, isCancel);
    await assert.rejects(
      getSoftware({ config: { ...config, signal: controller.signal } }),
      isCancel
    );
  });
});
