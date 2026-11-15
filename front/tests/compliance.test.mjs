import { QueryClient } from '@tanstack/react-query';
import { isCancel } from 'axios';
import assert from 'node:assert/strict';
import { Buffer } from 'node:buffer';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import './register.mjs';

const { acknowledgeSchema, findingsSearchSchema, myDeviceSearchSchema, readDashboard } =
  await import('../src/features/compliance/model.ts');
const { acknowledgeFinding, dashboardOptions } =
  await import('../src/features/compliance/queries.ts');
const { deviceDetailOptions, inventoryTableOptions, myDeviceOptions } =
  await import('../src/features/devices/queries.ts');
const { createDemoAuth } = await import('../src/app/auth/session.ts');
const { instance, setApiSession } = await import('../src/utils/lib/instance.ts');
const { getDataParams } = await import('../src/utils/api/data.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');

const id = '00000000-0000-4000-8000-000000000001';
const foreignId = '00000000-0000-4000-8000-000000000002';
const serial = 'VMware-42 37 a3 72';

test('риски: категории/статусы/экспорт и UUID валидируются; URL USER не выбирает чужое устройство', () => {
  for (const finding_type of ['FORBIDDEN_SOFTWARE', 'OUTDATED_SOFTWARE', 'LICENSE_DEFICIT']) {
    for (const status of ['OPEN', 'ACKNOWLEDGED', 'RESOLVED']) {
      const filters = findingsSearchSchema.parse({
        finding_type,
        status,
        severity: 'HIGH',
        serial_number: serial,
        product_id: id,
        product: ' Редактор '
      });
      const json = getDataParams('findings', filters);
      const xlsx = getDataParams('findings', filters, 'xlsx');
      assert.deepEqual({ ...json, format: 'xlsx' }, xlsx);
      assert.equal(json.status, status);
      assert.equal(json.product, 'Редактор');
    }
  }
  for (const input of [
    { product_id: 'bad' },
    { finding_type: '__proto__' },
    { status: 'DONE' },
    { severity: 'WARNING' },
    { limit: 0 }
  ])
    assert.equal(findingsSearchSchema.safeParse(input).success, false);
  assert.deepEqual(
    myDeviceSearchSchema.parse({ serial_number: 'foreign', device_id: foreignId, product_id: id }),
    { finding_type: 'FORBIDDEN_SOFTWARE' }
  );
  assert.equal(myDeviceSearchSchema.safeParse({ finding_type: 'LICENSE_DEFICIT' }).success, false);
  for (const comment of ['', '  \n\t'])
    assert.equal(acknowledgeSchema.safeParse({ comment }).success, false);
  assert.deepEqual(acknowledgeSchema.parse({ comment: ' Проверено ' }), { comment: 'Проверено' });
});

test('Dashboard сохраняет нули и отсутствие полей; отрицательные/нечисловые показатели отклоняются', () => {
  const zero = readDashboard({
    devices: 0,
    installations: 0,
    recognized_percent: 0,
    license_deficit: { seats: 0 },
    private: 'discard'
  });
  assert.equal(zero.devices, 0);
  assert.equal(zero.recognized_percent, 0);
  assert.equal(zero.pending_review, undefined);
  assert.equal(zero.license_deficit.products, undefined);
  assert.equal(zero.private, undefined);
  assert.deepEqual(readDashboard({}), {});
  for (const payload of [
    null,
    { devices: -1 },
    { devices: 1.5 },
    { devices: '1' },
    { recognized_percent: 101 },
    { idle_licenses: { seats: null } }
  ])
    assert.throws(() => readDashboard(payload), ApiError);
});

test('generated HTTP: acknowledge/ошибки/инвалидация/отмена и собственное устройство без утечки в USER-кеш', async (t) => {
  const requests = [];
  let mode = 'normal';
  let findingStatus = 'OPEN';
  let release;
  let started;
  const device = {
    id,
    serial_number: serial,
    responsible: 'private',
    location: 'private',
    imports: []
  };
  const server = createServer(async (request, response) => {
    const url = new URL(request.url, 'http://localhost');
    const entry = {
      path: url.pathname,
      method: request.method,
      params: Object.fromEntries(url.searchParams)
    };
    requests.push(entry);
    const json = (payload, status = 200) => {
      response.writeHead(status, { 'content-type': 'application/json' });
      response.end(JSON.stringify(payload));
    };
    if (url.pathname === '/dashboard') return json({ devices: 0, recognized_percent: 0 });
    if (url.pathname === '/me/device')
      return mode === 'no-claim'
        ? json({ error: { message: 'Нет device_serial' } }, 403)
        : mode === 'no-device'
          ? json({ error: { message: 'Не найдено' } }, 404)
          : json(device);
    if (url.pathname.startsWith('/data/')) {
      const serialValue = mode === 'foreign' ? 'foreign' : serial;
      const columns = [
        'finding_id',
        'serial_number',
        'status',
        'product_id',
        'comment',
        'license_status',
        'responsible'
      ];
      const row = [id, serialValue, findingStatus, null, 'private', 'DEFICIT 1', 'private'];
      if (mode === 'missing-id') {
        columns.shift();
        row.shift();
      }
      return json({
        columns,
        rows: [row],
        total: 2,
        shown: 1
      });
    }
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    entry.body = JSON.parse(Buffer.concat(chunks).toString());
    if (mode === 'error')
      return json({ error: { message: 'Комментарий не принят', request_id: 'ack-400' } }, 400);
    if (mode === 'network') return request.socket.destroy();
    if (mode === 'slow') {
      release = () => json({});
      started();
      return;
    }
    findingStatus = 'ACKNOWLEDGED';
    return json({ ignored_generic_body: true });
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
    () => dashboardOptions(auth),
    (error) => error.status === 403
  );
  await assert.rejects(
    acknowledgeFinding(auth, client, id, 'Test'),
    (error) => error.status === 403
  );
  assert.equal(requests.length, 0);
  const own = await client.fetchQuery(myDeviceOptions(auth));
  assert.equal(own.serial_number, serial);
  assert.equal(own.imports, undefined);
  assert.equal(own.responsible, undefined);
  await assert.rejects(
    client.fetchQuery(deviceDetailOptions(auth, foreignId)),
    (error) => error.status === 404
  );
  assert.equal(
    requests.some((r) => r.path.includes(foreignId)),
    false
  );
  const options = inventoryTableOptions(auth, 'findings', { serial_number: serial });
  const table = await client.fetchQuery(options);
  assert.equal(table.total, 2);
  assert.equal(table.columns.includes('comment'), false);
  assert.equal(table.columns.includes('license_status'), false);
  assert.equal(table.columns.includes('responsible'), false);
  assert.equal(JSON.stringify(client.getQueryData(options.queryKey)).includes('private'), false);
  mode = 'foreign';
  await assert.rejects(
    options.queryFn({ signal: new AbortController().signal }),
    /другого устройства/
  );
  for (const [value, status] of [
    ['no-claim', 403],
    ['no-device', 404]
  ]) {
    mode = value;
    await assert.rejects(
      myDeviceOptions(auth).queryFn({ signal: new AbortController().signal }),
      (error) => error.status === status
    );
  }
  mode = 'normal';
  await auth.selectRole('ADMIN');
  assert.equal((await client.fetchQuery(dashboardOptions(auth))).devices, 0);
  mode = 'missing-id';
  await assert.rejects(
    inventoryTableOptions(auth, 'findings', {}).queryFn({ signal: new AbortController().signal }),
    /не вернул ID нарушений/
  );
  mode = 'normal';
  const beforeInvalid = requests.length;
  for (const comment of ['', ' \n '])
    await assert.rejects(acknowledgeFinding(auth, client, id, comment), ApiError);
  await assert.rejects(acknowledgeFinding(auth, client, 'bad', 'Test'), ApiError);
  assert.equal(requests.length, beforeInvalid);
  for (const resource of [
    'findings',
    'devices',
    'me',
    'dashboard',
    'compliance',
    'summary',
    'audit'
  ])
    client.setQueryData([resource, 'fixture'], 'cached');
  await acknowledgeFinding(auth, client, id, ' Проверено ');
  assert.equal(requests.at(-1).body.comment, 'Проверено');
  for (const resource of [
    'findings',
    'devices',
    'me',
    'dashboard',
    'compliance',
    'summary',
    'audit'
  ])
    assert.equal(client.getQueryState([resource, 'fixture']).isInvalidated, true);
  assert.equal(findingStatus, 'ACKNOWLEDGED');
  for (const failure of ['error', 'network']) {
    mode = failure;
    const before = requests.length;
    await assert.rejects(
      acknowledgeFinding(auth, client, id, 'Не терять ввод'),
      (error) => failure === 'network' || (error.status === 400 && error.requestId === 'ack-400')
    );
    assert.equal(requests.length, before + 1);
    assert.equal(findingStatus, 'ACKNOWLEDGED');
  }
  mode = 'slow';
  const received = new Promise((resolve) => {
    started = resolve;
  });
  const staleOptions = dashboardOptions(auth);
  const pending = acknowledgeFinding(auth, client, id, 'Старая роль').catch((error) => error);
  await received;
  await auth.selectRole('USER');
  assert.equal(isCancel(await pending), true);
  release();
  assert.equal(client.getQueryCache().getAll().length, 0);
  const beforeStale = requests.length;
  await assert.rejects(staleOptions.queryFn({ signal: new AbortController().signal }), ApiError);
  assert.equal(requests.length, beforeStale);
});
