import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';

import type { DemoAuth } from '@/app/auth/session';
import type { PatchApiV1DevicesByIdData } from '@/generated/api/types.gen';
import type { DataFilters } from '@/utils/api/data';

import { getDeviceById } from '@/generated/api/requests/devices/getDeviceById.gen';
import { patchDeviceById } from '@/generated/api/requests/devices/patchDeviceById.gen';
import { fetchDataTable } from '@/utils/api/data';
import { ApiError } from '@/utils/api/errors';

import { readDevice } from './model';

export function inventoryTableOptions(
  auth: DemoAuth,
  dataset: 'devices' | 'findings' | 'installations',
  filters: DataFilters
) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: [dataset, session.role, session.epoch, 'table', filters],
    queryFn: ({ signal }) => {
      auth.requestConfig(session);
      return fetchDataTable(dataset, session.role, filters, { signal });
    }
  });
}

export function deviceDetailOptions(auth: DemoAuth, id: string) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: ['devices', session.role, session.epoch, 'detail', id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getDeviceById({ path: { id }, config: { signal } });
      return readDevice(response.data, session.role, id);
    }
  });
}

export async function saveDevice(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  body: NonNullable<PatchApiV1DevicesByIdData['body']>
) {
  const session = auth.requireReady();
  if (session.role !== 'ADMIN') throw new ApiError('Доступ запрещён.', { status: 403 });
  try {
    const response = await patchDeviceById({
      path: { id },
      body,
      config: auth.requestConfig(session)
    });
    const device = readDevice(response.data, session.role, id);
    auth.requestConfig(session);
    queryClient.setQueryData(['devices', session.role, session.epoch, 'detail', id], device);
    return device;
  } finally {
    if (auth.isCurrent(session.epoch))
      await Promise.all(
        ['devices', 'me', 'installations', 'findings', 'audit'].map((resource) =>
          queryClient.invalidateQueries({ queryKey: [resource] })
        )
      );
  }
}
