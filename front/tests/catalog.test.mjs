import { QueryClient } from '@tanstack/react-query';
import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import './register.mjs';

const { getSoftwareParams, readProduct, readSoftwarePage, softwareSearchSchema } =
  await import('../src/features/software/model.ts');
const { softwareDetailOptions, softwareListOptions } =
  await import('../src/features/software/queries.ts');
const { contractsSearchSchema, CONTRACT_STATUSES, getContractLabel, LICENSE_METRICS } =
  await import('../src/features/contracts/model.ts');
const { createDemoAuth } = await import('../src/app/auth/session.ts');
const { instance, setApiSession } = await import('../src/utils/lib/instance.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');

const product = {
  id: 'f5b5fc3a-f23f-4e8d-a4ba-a2b3c4d5e6f7',
  canonical_name: '<script>Название продукта</script>',
  current_version: null,
  license_type: 'OPEN_SOURCE',
  cost_model: 'FREE',
  in_bpo: false,
  status: 'FORBIDDEN',
  status_comment: 'Комментарий',
  purchased_seats: 0,
  active_installations: 2,
  devices: 1,
  agreement_id: null,
  archived_at: null
};
const pagination = { page: 1, limit: 50, total: 1, total_pages: 1 };

test('пустые текстовые поля стенда допускают null без потери нулевых счётчиков', () => {
  for (const role of ['ADMIN', 'USER']) {
    const parsed = readProduct({ ...product, edition: null, status_comment: null }, role);
    assert.equal(parsed.edition, undefined);
    assert.equal(parsed.status_comment, undefined);
    assert.equal(parsed.canonical_name, product.canonical_name);
    assert.equal(parsed.purchased_seats, role === 'ADMIN' ? 0 : undefined);
  }
  assert.throws(() => readProduct({ ...product, edition: 42 }, 'ADMIN'), ApiError);
});

test('каталог нормализует все URL-фильтры, сохраняет false, ограничивает page/limit', () => {
  const search = softwareSearchSchema.parse({
    page: '2',
    limit: '20',
    search: ' office ',
    section: 'Раздел',
    class: 'Класс',
    license_type: 'COMMERCIAL',
    cost_model: 'PAID',
    status: 'ACTUAL',
    in_bpo: 'false',
    include_archived: 'true',
    ignored: 'private'
  });
  assert.deepEqual(getSoftwareParams(search, 'ADMIN'), {
    page: 2,
    limit: 20,
    search: 'office',
    section: 'Раздел',
    class: 'Класс',
    license_type: 'COMMERCIAL',
    cost_model: 'PAID',
    status: 'ACTUAL',
    in_bpo: false,
    include_archived: true
  });
  assert.equal(getSoftwareParams(search, 'USER').include_archived, undefined);
  assert.equal(softwareSearchSchema.parse({ in_bpo: true }).in_bpo, true);
  const cleared = softwareSearchSchema.parse({
    search: '  ',
    license_type: '',
    cost_model: '',
    status: '',
    in_bpo: '',
    include_archived: ''
  });
  assert.equal(cleared.page, 1);
  assert.equal(cleared.limit, 50);
  for (const key of [
    'search',
    'license_type',
    'cost_model',
    'status',
    'in_bpo',
    'include_archived'
  ])
    assert.equal(cleared[key], undefined);
  for (const invalid of [
    { page: 0 },
    { page: 1.5 },
    { limit: 201 },
    { limit: 0 },
    { in_bpo: '0' },
    { status: 'FUTURE' },
    { license_type: 'OTHER' }
  ])
    assert.equal(softwareSearchSchema.safeParse(invalid).success, false);
});

test('проекция USER удаляет служебные поля даже из ADMIN-ответа перед кешированием', () => {
  const user = readProduct({ ...product, unlisted: 'private' }, 'USER');
  assert.equal(user.in_bpo, false);
  assert.equal(user.current_version, null);
  assert.equal(user.canonical_name, product.canonical_name);
  for (const field of [
    'purchased_seats',
    'active_installations',
    'devices',
    'agreement_id',
    'archived_at',
    'unlisted'
  ])
    assert.equal(Object.hasOwn(user, field), false);
  assert.equal(readProduct(product, 'ADMIN').purchased_seats, 0);
  assert.equal(
    readSoftwarePage({ items: [product], pagination }, 'USER').items[0].purchased_seats,
    undefined
  );
});

test('невалидные карточки и списки являются ошибкой контракта, а не пустыми данными', () => {
  for (const invalid of [
    null,
    {},
    { ...product, id: 'bad' },
    { ...product, status: 'FUTURE' },
    { ...product, in_bpo: 'false' },
    { ...product, purchased_seats: -1 }
  ])
    assert.throws(() => readProduct(invalid, 'ADMIN'), ApiError);
  for (const invalid of [
    null,
    {},
    { items: [{}], pagination },
    { items: [product, product], pagination: { ...pagination, total: 2 } },
    { items: [product], pagination: { ...pagination, total: 0 } },
    { items: [product], pagination: { ...pagination, total: 100, total_pages: 1 } },
    { items: [], pagination: { ...pagination, limit: 0 } }
  ])
    assert.throws(() => readSoftwarePage(invalid, 'ADMIN'), ApiError);
  assert.equal(
    readSoftwarePage({ items: [], pagination: { ...pagination, total: 0, total_pages: 0 } }, 'USER')
      .items.length,
    0
  );
});

test('договоры сохраняют только документированные фильтры без paginator, все статусы и метрики различимы', () => {
  assert.deepEqual(
    contractsSearchSchema.parse({
      product: ' SAM ',
      product_id: product.id,
      contract_number: ' №1 ',
      metric: 'PER_DEVICE',
      status: 'EXPIRING',
      limit: '5',
      page: 2
    }),
    {
      product: 'SAM',
      product_id: product.id,
      contract_number: '№1',
      metric: 'PER_DEVICE',
      status: 'EXPIRING',
      limit: 5
    }
  );
  assert.equal(contractsSearchSchema.parse({ product_id: '' }).product_id, undefined);
  for (const invalid of [
    { product_id: 'bad' },
    { status: 'ACTUAL' },
    { metric: 'OTHER' },
    { limit: 0 }
  ])
    assert.equal(contractsSearchSchema.safeParse(invalid).success, false);
  assert.equal(new Set(Object.values(CONTRACT_STATUSES)).size, 5);
  for (const [value, label] of Object.entries(CONTRACT_STATUSES))
    assert.equal(getContractLabel('status', value), label);
  for (const [value, label] of Object.entries(LICENSE_METRICS))
    assert.equal(getContractLabel('metric', value), label);
  assert.equal(getContractLabel('status', 'toString'), undefined);
  assert.equal(getContractLabel('quantity', 0), undefined);
});

test('настоящий generated-клиент передаёт фильтры, UUID, отмену и изолирует кеш по роли/контексту', async (t) => {
  const requests = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url, 'http://localhost');
    requests.push({ path: url.pathname, params: Object.fromEntries(url.searchParams) });
    if (url.pathname.endsWith('/00000000-0000-4000-8000-000000000000')) {
      response.writeHead(404, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ error: { code: 'NOT_FOUND', message: 'Объект недоступен.' } }));
      return;
    }
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(
      JSON.stringify(url.pathname === '/software' ? { items: [product], pagination } : product)
    );
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
  let token = null;
  const storage = {
    getItem: () => token,
    setItem: (_key, value) => {
      token = value;
    }
  };
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: storage });
  const baseURL = instance.defaults.baseURL;
  const adapter = instance.defaults.adapter;
  instance.defaults.baseURL = `http://127.0.0.1:${server.address().port}`;
  instance.defaults.adapter = 'fetch';
  const auth = createDemoAuth(client, () => storage);
  setApiSession(auth);
  t.after(() => {
    setApiSession();
    client.clear();
    instance.defaults.baseURL = baseURL;
    instance.defaults.adapter = adapter;
    if (descriptor) Object.defineProperty(globalThis, 'localStorage', descriptor);
    else delete globalThis.localStorage;
  });
  await auth.initialize();
  const search = softwareSearchSchema.parse({
    search: ' SAM ',
    in_bpo: false,
    include_archived: true
  });
  const userOptions = softwareListOptions(auth, search);
  const user = await client.fetchQuery(userOptions);
  assert.equal(user.items[0].purchased_seats, undefined);
  assert.deepEqual(requests[0], {
    path: '/software',
    params: { page: '1', limit: '50', search: 'SAM', in_bpo: 'false' }
  });
  await client.fetchQuery(softwareDetailOptions(auth, product.id));
  assert.equal(requests[1].path, `/software/${product.id}`);
  await assert.rejects(
    client.fetchQuery(softwareDetailOptions(auth, '00000000-0000-4000-8000-000000000000')),
    (error) => error.status === 404
  );
  await auth.selectRole('ADMIN');
  const adminOptions = softwareListOptions(auth, search);
  assert.notDeepEqual(adminOptions.queryKey, userOptions.queryKey);
  assert.equal((await client.fetchQuery(adminOptions)).items[0].purchased_seats, 0);
  assert.equal(requests.at(-1).params.include_archived, 'true');
  assert.equal(client.getQueryData(userOptions.queryKey), undefined);
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    adminOptions.queryFn({ signal: controller.signal }),
    (error) => error.code === 'ERR_CANCELED'
  );
  await auth.selectRole('USER');
  const count = requests.length;
  await assert.rejects(adminOptions.queryFn({ signal: new AbortController().signal }), ApiError);
  assert.equal(requests.length, count);
});
