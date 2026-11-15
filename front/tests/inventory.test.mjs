import { QueryClient } from '@tanstack/react-query';
import { isCancel } from 'axios';
import assert from 'node:assert/strict';
import { Buffer } from 'node:buffer';
import { once } from 'node:events';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { test } from 'node:test';

import './register.mjs';

const {
  IMPORT_STATUSES,
  canRetryImport,
  getCollectedAt,
  getCsvFileError,
  importsSearchSchema,
  isImportTerminal,
  readImport,
  readImportPreview,
  readImportsPage
} = await import('../src/features/imports/model.ts');
const {
  importDetailOptions,
  importPreviewOptions,
  importsListOptions,
  refreshInventory,
  runImportAction,
  uploadCsv
} = await import('../src/features/imports/queries.ts');
const { deviceFormSchema, devicesSearchSchema, getDeviceFormValues, readDevice } =
  await import('../src/features/devices/model.ts');
const { deviceDetailOptions, inventoryTableOptions, saveDevice } =
  await import('../src/features/devices/queries.ts');
const { installationsSearchSchema } = await import('../src/features/installations/model.ts');
const { createDemoAuth } = await import('../src/app/auth/session.ts');
const { instance, setApiSession } = await import('../src/utils/lib/instance.ts');
const { getDataParams, projectDataTable } = await import('../src/utils/api/data.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');

const id = '00000000-0000-4000-8000-000000000001';
const otherId = '00000000-0000-4000-8000-000000000002';
const serial = 'VMware-42 37 a3 72';
const snapshot = {
  id,
  filename: 'inventory.csv',
  status: 'AWAITING_CONFIRMATION',
  total_rows: 4,
  loaded_rows: 0,
  duplicate_rows: 0,
  excluded_rows: 0,
  error_rows: 0,
  failed_devices: 0,
  collected_at: '2026-11-15T10:00:00+05:00'
};
const preview = {
  delimiter: ',',
  columns: ['COMPUTERS.SERIAL_NUMBER', 'SOFTWARE.DISPLAYNAME'],
  rows: [
    {
      row: 2,
      serial_number: serial,
      display_name: 'Редактор, корпоративный',
      version: '2.0',
      excluded: false
    },
    { row: 4, serial_number: serial, display_name: 'Security Update (KB5030000)', excluded: true },
    { row: 5, error: 'Не заполнен серийный номер' }
  ]
};
const device = {
  id,
  serial_number: serial,
  device_type: 'VIRTUAL',
  status: 'NEW',
  hostname: null,
  responsible: null,
  location: null,
  operating_system: 'Linux',
  installations_count: 0,
  open_findings_count: 0,
  imports: [{ inventory_id: otherId, result: 'SUPERSEDED', collected_at: snapshot.collected_at }]
};

test('восемь статусов импорта, терминальные состояния и retry не смешиваются с ошибками CSV', () => {
  assert.equal(Object.keys(IMPORT_STATUSES).length, 8);
  for (const status of Object.keys(IMPORT_STATUSES)) {
    assert.equal(readImport({ ...snapshot, status }, id).status, status);
    assert.equal(
      isImportTerminal(status),
      ['CANCELLED', 'COMPLETED', 'COMPLETED_WITH_ERRORS', 'FAILED'].includes(status)
    );
  }
  assert.equal(canRetryImport({ status: 'COMPLETED_WITH_ERRORS', failed_devices: 1 }), true);
  assert.equal(canRetryImport({ status: 'FAILED', failed_devices: 1 }), true);
  for (const value of [
    { status: 'FAILED' },
    { status: 'COMPLETED', failed_devices: 1 },
    { status: 'PROCESSING', failed_devices: 1 },
    { status: 'COMPLETED_WITH_ERRORS', failed_devices: 0, error_rows: 3 }
  ])
    assert.equal(canRetryImport(value), false);
});

test('Import/preview/history проверяют DTO, ID и счётчики, сохраняют нули и ошибки предпросмотра', () => {
  assert.equal(readImport({ ...snapshot, private: 'discard' }).private, undefined);
  assert.equal(readImport(snapshot).loaded_rows, 0);
  assert.equal(readImportPreview(preview).rows[1].excluded, true);
  assert.equal(readImportPreview(preview).rows[2].error, 'Не заполнен серийный номер');
  for (const invalid of [
    {},
    { ...snapshot, id: otherId },
    { ...snapshot, status: 'UNKNOWN' },
    { ...snapshot, loaded_rows: -1 },
    { ...snapshot, progress_percent: 101 },
    { ...snapshot, collected_at: 'yesterday' }
  ])
    assert.throws(() => readImport(invalid, id), ApiError);
  assert.throws(() => readImportPreview({ ...preview, rows: [{ row: '2' }] }), ApiError);
  const pagination = { page: 1, limit: 50, total: 1, total_pages: 1 };
  assert.equal(readImportsPage({ items: [snapshot], pagination }).items[0].id, id);
  assert.throws(() => readImportsPage({ items: [snapshot, snapshot], pagination }), ApiError);
  assert.throws(
    () => readImportsPage({ items: [snapshot], pagination: { ...pagination, total_pages: 0 } }),
    ApiError
  );
  assert.equal(importsSearchSchema.parse({ page: '2', limit: '1' }).page, 2);
  assert.equal(importsSearchSchema.safeParse({ limit: 201 }).success, false);
});

test('CSV проверяется до отправки без парсинга файла; локальная дата отправляется с часовым поясом', () => {
  for (const file of [
    undefined,
    { name: 'x.csv', size: 0 },
    { name: 'x.exe', size: 1 },
    { name: 'x.csv', size: 50_000_001 }
  ])
    assert.equal(typeof getCsvFileError(file), 'string');
  assert.equal(getCsvFileError({ name: 'ИНВЕНТАРЬ.CSV', size: 50_000_000 }), undefined);
  assert.equal(getCollectedAt(''), undefined);
  const localTime = '2026-11-15T12:30';
  assert.equal(new Date(getCollectedAt(localTime)).getTime(), new Date(localTime).getTime());
  for (const invalid of ['tomorrow', '2026-02-30T10:00', '2026-11-15T25:00'])
    assert.throws(() => getCollectedAt(invalid), ApiError);
});

test('карточка устройства сохраняет VM-серийник и удаляет служебные поля до USER-кеша', () => {
  const own = readDevice(device, 'USER', id);
  assert.equal(own.serial_number, serial);
  assert.equal(own.installations_count, 0);
  for (const name of ['imports', 'responsible', 'location']) assert.equal(own[name], undefined);
  assert.equal(readDevice(device, 'ADMIN', id).imports[0].result, 'SUPERSEDED');
  for (const result of ['APPLIED', 'ALREADY_PROCESSED', 'SUPERSEDED'])
    assert.equal(
      readDevice({ ...device, imports: [{ inventory_id: otherId, result }] }, 'ADMIN', id)
        .imports[0].result,
      result
    );
  for (const invalid of [
    { ...device, id: otherId },
    { ...device, serial_number: '' },
    { ...device, installations_count: -1 },
    { ...device, imports: [{ inventory_id: 'fake', result: 'APPLIED' }] }
  ])
    assert.throws(() => readDevice(invalid, 'ADMIN', id), ApiError);
  const body = deviceFormSchema.parse({
    ...getDeviceFormValues(device),
    hostname: ' pc-1 ',
    responsible: '',
    location: '  ',
    status: 'ACTIVE',
    serial_number: 'tampered',
    operating_system: 'tampered'
  });
  assert.deepEqual(body, {
    device_type: 'VIRTUAL',
    status: 'ACTIVE',
    hostname: 'pc-1',
    responsible: null,
    location: null
  });
});

test('фильтры устройств/установок сохраняют false, полный VM-серийник и только документированные группы', () => {
  const devices = devicesSearchSchema.parse({
    serial_number: serial,
    status: 'NEW',
    limit: '10',
    responsible: '   '
  });
  assert.equal(getDataParams('devices', devices).serial_number, serial);
  assert.equal(devices.responsible, undefined);
  const filters = installationsSearchSchema.parse({
    serial_number: serial,
    recognized: 'false',
    in_bpo: false,
    group_by: 'product',
    product_id: id
  });
  assert.equal(getDataParams('installations', filters).recognized, false);
  assert.equal(getDataParams('installations', filters).in_bpo, false);
  assert.equal(installationsSearchSchema.safeParse({ group_by: 'device_id' }).success, false);
  assert.equal(installationsSearchSchema.safeParse({ product_id: 'invalid' }).success, false);
  assert.equal(devicesSearchSchema.safeParse({ status: 'DELETED' }).success, false);
});

test('generated HTTP: CSV/preview/confirm/cancel/retry, ошибки 400/409/413, PATCH и смена роли', async (t) => {
  const requests = [];
  let stored = { ...snapshot };
  let storedDevice = { ...device };
  let malformed = false;
  let started;
  let finish;
  const server = createServer(async (request, response) => {
    const url = new URL(request.url, 'http://localhost');
    const entry = {
      method: request.method,
      path: url.pathname,
      params: Object.fromEntries(url.searchParams)
    };
    requests.push(entry);
    const json = (value, status = 200) => {
      response.writeHead(status, { 'content-type': 'application/json' });
      response.end(JSON.stringify(value));
    };
    try {
      assert.ok(request.headers.authorization?.startsWith('Bearer '));
      if (entry.path === '/imports' && request.method === 'POST') {
        const chunks = [];
        for await (const chunk of request) chunks.push(chunk);
        const form = await new Request('http://localhost', {
          method: 'POST',
          headers: request.headers,
          body: Buffer.concat(chunks)
        }).formData();
        const file = form.get('file');
        entry.file = { name: file.name, text: await file.text() };
        assert.match(request.headers['content-type'], /^multipart\/form-data; boundary=/);
        if (file.name.includes('invalid-headers'))
          return json(
            { error: { message: 'Нет обязательных заголовков', request_id: 'csv-400' } },
            400
          );
        if (file.name === 'oversize.csv')
          return json({ error: { message: 'CSV слишком большой' } }, 413);
        stored = { ...snapshot, filename: file.name };
        return json(
          {
            import_id: id,
            status: stored.status,
            filename: file.name,
            preview: { ...preview, delimiter: file.name.includes('semicolon') ? ';' : ',' }
          },
          202
        );
      }
      if (entry.path.endsWith('/preview')) return json(preview);
      if (entry.path.endsWith('/confirm')) {
        if (stored.status !== 'AWAITING_CONFIRMATION')
          return json({ error: { message: 'Уже подтверждён' } }, 409);
        stored = { ...stored, status: 'PROCESSING' };
      }
      if (entry.path.endsWith('/cancel')) stored = { ...stored, status: 'CANCELLED' };
      if (entry.path.endsWith('/retry')) {
        stored = { ...stored, status: 'PENDING' };
        return json(stored, 202);
      }
      if (entry.path === '/imports')
        return json({
          items: [stored],
          pagination: {
            page: Number(url.searchParams.get('page')),
            limit: Number(url.searchParams.get('limit')),
            total: 1,
            total_pages: 1
          }
        });
      if (entry.path === `/devices/${id}`) {
        if (request.method === 'PATCH') {
          const chunks = [];
          for await (const chunk of request) chunks.push(chunk);
          entry.body = JSON.parse(Buffer.concat(chunks).toString());
          storedDevice = { ...storedDevice, ...entry.body };
        }
        return json(storedDevice);
      }
      if (entry.path === '/data/installations')
        return json({
          columns: ['raw_name', 'product_id', 'version', 'serial_number'],
          rows: [['Неизвестная программа', null, null, serial]],
          total: 2,
          shown: 1
        });
      if (started) {
        finish = () => json(stored);
        started();
        started = undefined;
        return;
      }
      return json(malformed ? { ...stored, id: otherId } : stored);
    } catch (error) {
      return json({ error: { message: error.message } }, 500);
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let token;
  const storage = {
    getItem: () => token,
    setItem: (_key, value) => {
      token = value;
    }
  };
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage });
  const { baseURL, adapter } = instance.defaults;
  instance.defaults.baseURL = `http://127.0.0.1:${server.address().port}`;
  instance.defaults.adapter = 'http';
  const auth = createDemoAuth(client, () => storage);
  setApiSession(auth);
  t.after(() => {
    setApiSession();
    client.clear();
    Object.assign(instance.defaults, { baseURL, adapter });
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor);
    else delete globalThis.localStorage;
  });
  await auth.initialize();
  assert.throws(
    () => importDetailOptions(auth, id),
    (error) => error.status === 403
  );
  await assert.rejects(
    runImportAction(auth, client, id, 'confirm'),
    (error) => error.status === 403
  );
  await assert.rejects(saveDevice(auth, client, id, {}), (error) => error.status === 403);
  assert.equal(requests.length, 0);
  await auth.selectRole('ADMIN');
  for (const name of ['inventory-comma.csv', 'inventory-semicolon.csv']) {
    const text = await readFile(new URL(`./fixtures/${name}`, import.meta.url), 'utf8');
    const uploaded = await uploadCsv(auth, client, new File([text], name), snapshot.collected_at);
    assert.equal(uploaded.import_id, id);
    assert.equal(uploaded.status, 'AWAITING_CONFIRMATION');
    assert.equal(requests.at(-1).params.confirm, 'false');
    assert.equal(requests.at(-1).params.collected_at, snapshot.collected_at);
    assert.equal(requests.at(-1).file.text, text);
    assert.equal(storedDevice.installations_count, 0);
    assert.equal(uploaded.preview.delimiter, name.includes('semicolon') ? ';' : ',');
  }
  const badHeaders = await readFile(
    new URL('./fixtures/inventory-invalid-headers.csv', import.meta.url),
    'utf8'
  );
  await assert.rejects(
    uploadCsv(auth, client, new File([badHeaders], 'inventory-invalid-headers.csv')),
    (error) => error.status === 400 && error.requestId === 'csv-400'
  );
  await assert.rejects(
    uploadCsv(auth, client, new File(['x'], 'oversize.csv')),
    (error) => error.status === 413
  );
  const beforeInvalid = requests.length;
  await assert.rejects(uploadCsv(auth, client, new File(['x'], 'x.csv'), 'invalid'), ApiError);
  assert.equal(requests.length, beforeInvalid);
  const detail = importDetailOptions(auth, id);
  assert.equal((await client.fetchQuery(detail)).id, id);
  assert.equal((await client.fetchQuery(importPreviewOptions(auth, id))).rows[1].excluded, true);
  const listed = await client.fetchQuery(importsListOptions(auth, { page: 1, limit: 2 }));
  assert.equal(listed.pagination.limit, 2);
  assert.equal((await runImportAction(auth, client, id, 'confirm')).status, 'PROCESSING');
  const beforeConflict = requests.length;
  await assert.rejects(
    runImportAction(auth, client, id, 'confirm'),
    (error) => error.status === 409
  );
  assert.equal(requests.length, beforeConflict + 1);
  stored = {
    ...stored,
    status: 'COMPLETED_WITH_ERRORS',
    loaded_rows: 1,
    duplicate_rows: 1,
    excluded_rows: 1,
    error_rows: 1,
    failed_devices: 1
  };
  const completed = await detail.queryFn({ signal: new AbortController().signal });
  assert.equal(completed.loaded_rows, 1);
  assert.equal(completed.duplicate_rows, 1);
  assert.equal(completed.excluded_rows, 1);
  assert.equal(completed.error_rows, 1);
  assert.equal((await runImportAction(auth, client, id, 'retry')).status, 'PENDING');
  assert.equal(stored.collected_at, snapshot.collected_at);
  stored = { ...snapshot };
  assert.equal((await runImportAction(auth, client, id, 'cancel')).status, 'CANCELLED');
  malformed = true;
  await assert.rejects(
    detail.queryFn({ signal: new AbortController().signal }),
    (error) => error.code === 'INVALID_RESPONSE'
  );
  malformed = false;
  for (const resource of [
    'devices',
    'installations',
    'software',
    'dashboard',
    'balance',
    'findings',
    'audit'
  ])
    client.setQueryData([resource, 'ADMIN', auth.requireReady().epoch], 'cached');
  await refreshInventory(client);
  assert.equal(
    client.getQueryState(['software', 'ADMIN', auth.requireReady().epoch]).isInvalidated,
    true
  );
  const body = deviceFormSchema.parse({
    ...getDeviceFormValues(device),
    status: 'ACTIVE',
    responsible: ' Администратор ',
    location: ''
  });
  const saved = await saveDevice(auth, client, id, body);
  assert.equal(saved.status, 'ACTIVE');
  assert.equal(saved.serial_number, serial);
  assert.equal(requests.at(-1).body.responsible, 'Администратор');
  assert.equal(requests.at(-1).body.location, null);
  assert.equal((await client.fetchQuery(deviceDetailOptions(auth, id))).status, 'ACTIVE');
  const data = await client.fetchQuery(
    inventoryTableOptions(
      auth,
      'installations',
      installationsSearchSchema.parse({ recognized: false, serial_number: serial })
    )
  );
  assert.equal(requests.at(-1).params.recognized, 'false');
  assert.equal(requests.at(-1).params.serial_number, serial);
  assert.equal(projectDataTable('installations', 'ADMIN', data).rows[0][0], null);
  assert.equal(projectDataTable('installations', 'ADMIN', data).total, 2);
  const state = { data: { ...snapshot, status: 'PROCESSING' }, fetchFailureCount: 0, error: null };
  assert.equal(detail.refetchInterval({ state }), 2000);
  assert.equal(
    detail.refetchInterval({
      state: { ...state, fetchFailureCount: 3, error: new ApiError('network') }
    }),
    16000
  );
  for (const status of [
    'AWAITING_CONFIRMATION',
    'COMPLETED',
    'COMPLETED_WITH_ERRORS',
    'CANCELLED',
    'FAILED'
  ])
    assert.equal(
      detail.refetchInterval({ state: { ...state, data: { ...snapshot, status } } }),
      false
    );
  assert.equal(
    detail.refetchInterval({ state: { ...state, error: new ApiError('404', { status: 404 }) } }),
    false
  );
  assert.equal(
    detail.refetchInterval({
      state: { ...state, error: new ApiError('contract', { code: 'INVALID_RESPONSE' }) }
    }),
    false
  );
  const received = new Promise((resolve) => {
    started = resolve;
  });
  const delayed = detail.queryFn({ signal: new AbortController().signal }).catch((error) => error);
  await received;
  await auth.selectRole('USER');
  assert.equal(isCancel(await delayed), true);
  finish();
  assert.equal(client.getQueryCache().getAll().length, 0);
  const beforeStale = requests.length;
  await assert.rejects(detail.queryFn({ signal: new AbortController().signal }), ApiError);
  assert.equal(requests.length, beforeStale);
  assert.equal(detail.refetchInterval({ state }), false);
  assert.equal((await client.fetchQuery(deviceDetailOptions(auth, id))).imports, undefined);
});
