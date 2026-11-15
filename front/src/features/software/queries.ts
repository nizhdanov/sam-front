import { queryOptions } from '@tanstack/react-query';

import type { DemoAuth } from '@/app/auth/session';

import { getSoftware } from '@/generated/api/requests/software/getSoftware.gen';
import { getSoftwareById } from '@/generated/api/requests/software/getSoftwareById.gen';

import type { SoftwareSearch } from './model';

import { getSoftwareParams, readProduct, readSoftwarePage } from './model';

export function softwareListOptions(auth: DemoAuth, search: SoftwareSearch) {
  const session = auth.requireReady();
  const params = getSoftwareParams(search, session.role);
  return queryOptions({
    queryKey: ['software', session.role, session.epoch, params],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getSoftware({ query: params, config: { signal } });
      return readSoftwarePage(response.data, session.role);
    }
  });
}

export function softwareDetailOptions(auth: DemoAuth, id: string) {
  const session = auth.requireReady();
  return queryOptions({
    queryKey: ['software', session.role, session.epoch, id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getSoftwareById({ path: { id }, config: { signal } });
      return readProduct(response.data, session.role);
    }
  });
}
