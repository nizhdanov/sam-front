import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import { test } from 'node:test';

import './register.mjs';

const { projectDataTable, getDataParams, formatDataValue, fetchDataTable, exportDataTable } =
  await import('../src/utils/api/data.ts');
const { ApiError } = await import('../src/utils/api/errors.ts');

test('колонки связываются по имени, неизвестные/ADMIN-колонки не попадают в USER UI', () => {
  const payload = {
    dataset: 'products',
    columns: ['purchased', 'untrusted', 'product', 'in_bpo'],
    rows: [[100, { secret: 'private' }, 'SAM', false]],
    total: 12,
    shown: 1
  };
  assert.deepEqual(projectDataTable('products', 'USER', payload), {
    columns: ['product', 'in_bpo'],
    rows: [['SAM', false]],
    total: 12,
    shown: 1
  });
  assert.deepEqual(projectDataTable('products', 'ADMIN', payload).rows, [['SAM', false, 100]]);
  const missing = { columns: ['product'], rows: [['SAM']], shown: 1, total: 1 };
  assert.deepEqual(projectDataTable('products', 'USER', missing).rows, [['SAM']]);
  assert.throws(
    () => projectDataTable('contracts', 'USER', payload),
    (error) => error.status === 403
  );
  assert.throws(
    () => projectDataTable('products', null, payload),
    (error) => error.status === 403
  );
});

test('несогласованная таблица является ошибкой, а не пустым результатом', () => {
  const valid = { columns: ['serial_number'], rows: [['VMware-42 37 a3 72']], total: 1, shown: 1 };
  for (const invalid of [
    null,
    {},
    { ...valid, columns: [] },
    { ...valid, columns: ['unknown'] },
    { ...valid, columns: ['serial_number', 'serial_number'] },
    { ...valid, rows: [[]] },
    { ...valid, rows: [['a', 'b']] },
    { ...valid, shown: 0 },
    { ...valid, total: 0 },
    { ...valid, total: -1 },
    { ...valid, dataset: 'products' },
    { ...valid, group_by: 'status' }
  ])
    assert.throws(() => projectDataTable('devices', 'ADMIN', invalid), ApiError);
  assert.equal(projectDataTable('devices', 'ADMIN', valid).rows[0][0], 'VMware-42 37 a3 72');
  assert.deepEqual(
    projectDataTable('devices', 'ADMIN', { ...valid, rows: [], total: 0, shown: 0 }).rows,
    []
  );
});

test('группировка разрешена только по контракту, значение и count связываются по именам', () => {
  const payload = {
    group_by: 'device_type',
    columns: ['count', 'device_type'],
    rows: [[5, 'VIRTUAL']],
    shown: 1,
    total: 1
  };
  assert.deepEqual(projectDataTable('devices', 'USER', payload, 'device_type').rows, [
    ['VIRTUAL', 5]
  ]);
  assert.throws(() => projectDataTable('devices', 'ADMIN', payload, 'hostname'), ApiError);
  assert.throws(
    () =>
      projectDataTable(
        'devices',
        'ADMIN',
        { ...payload, columns: ['device_type'], rows: [['VIRTUAL']] },
        'device_type'
      ),
    ApiError
  );
});

test('неизвестные значения не исполняются и не сериализуют произвольные объекты', () => {
  assert.equal(formatDataValue(null), 'Не указано');
  assert.equal(formatDataValue(false), 'Нет');
  assert.equal(formatDataValue(0), '0');
  assert.equal(formatDataValue('<script>private</script>'), '<script>private</script>');
  assert.equal(formatDataValue('FUTURE_STATUS'), 'FUTURE_STATUS');
  for (const value of [{ secret: 'private' }, ['private'], NaN, Infinity]) {
    assert.equal(formatDataValue(value), 'Неизвестное значение');
  }
});

test('whitelist фильтров сохраняет recognized/in_bpo=false, но не выдумывает page/offset/sort', () => {
  const filters = {
    recognized: false,
    in_bpo: 'false',
    raw_name: ' studio ',
    group_by: 'product',
    limit: '20',
    publisher: '',
    status: null
  };
  assert.deepEqual(getDataParams('installations', filters), {
    format: 'json',
    limit: 20,
    recognized: false,
    in_bpo: false,
    raw_name: 'studio',
    group_by: 'product'
  });
  assert.equal(getDataParams('products', { in_bpo: true }).in_bpo, true);
  for (const invalid of [
    { page: 2 },
    { offset: 100 },
    { sort: 'status' },
    { recognized: '0' },
    { limit: 0 },
    { limit: 1.5 },
    { limit: '1.5' },
    { product_id: 'fake' },
    { group_by: 'hostname' }
  ]) {
    assert.throws(() => getDataParams('installations', invalid), ApiError);
  }
  assert.throws(() => getDataParams('devices', { recognized: false }), ApiError);
  assert.equal(getDataParams('devices', {}, 'csv').limit, 10_000);
  assert.throws(() => getDataParams('devices', { limit: 10_001 }, 'xlsx'), ApiError);
});

test('generated data/export используют одинаковые фильтры и группировку, файлы передаются браузеру', async (t) => {
  const queries = [];
  const server = createServer((request, response) => {
    const url = new URL(request.url, 'http://localhost');
    queries.push(Object.fromEntries(url.searchParams));
    if (url.searchParams.get('raw_name') === 'invalid') {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end('null');
      return;
    }
    if (url.searchParams.get('format') === 'json') {
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(
        JSON.stringify({ columns: ['product', 'count'], rows: [['SAM', 1]], shown: 1, total: 1 })
      );
    } else {
      const csv = url.searchParams.get('format') === 'csv';
      response.writeHead(200, {
        'content-type': csv
          ? 'text/csv'
          : 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
      });
      response.end(csv ? 'product,count\nSAM,1' : 'xlsx fixture');
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
  const storageDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { getItem: () => null }
  });
  const downloaded = [];
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: {
      createElement: () => ({
        click() {
          downloaded.push(this.download);
        },
        remove() {}
      }),
      body: { append() {} }
    }
  });
  t.after(() => {
    delete globalThis.document;
    if (storageDescriptor) Object.defineProperty(globalThis, 'localStorage', storageDescriptor);
    else delete globalThis.localStorage;
  });
  const filters = { recognized: false, raw_name: 'studio', group_by: 'product', limit: 5 };
  const config = { baseURL: `http://127.0.0.1:${server.address().port}`, adapter: 'fetch' };
  const data = await fetchDataTable('installations', 'ADMIN', filters, config);
  assert.equal(projectDataTable('installations', 'ADMIN', data, 'product').shown, 1);
  await exportDataTable('installations', 'ADMIN', filters, 'csv', config);
  await exportDataTable('installations', 'ADMIN', filters, 'xlsx', config);
  for (const [index, format] of ['json', 'csv', 'xlsx'].entries()) {
    assert.deepEqual(queries[index], {
      ...getDataParams('installations', filters, format),
      limit: '5',
      recognized: 'false'
    });
  }
  assert.deepEqual(downloaded, ['installations.csv', 'installations.xlsx']);
  await assert.rejects(
    exportDataTable('contracts', 'USER', {}, 'csv', config),
    (error) => error.status === 403
  );
  assert.equal(queries.length, 3);
  await assert.rejects(
    fetchDataTable('installations', 'ADMIN', { raw_name: 'invalid' }, config),
    ApiError
  );
});
