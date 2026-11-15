import type { QueryClient } from '@tanstack/react-query';

import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';
import type { ProductInput } from '@/generated/api/types.gen';

import { deleteSoftwareById } from '@/generated/api/requests/software/deleteSoftwareById.gen';
import { getSoftware } from '@/generated/api/requests/software/getSoftware.gen';
import { getSoftwareById } from '@/generated/api/requests/software/getSoftwareById.gen';
import { getSoftwareByIdVersions } from '@/generated/api/requests/software/getSoftwareByIdVersions.gen';
import { patchSoftwareById } from '@/generated/api/requests/software/patchSoftwareById.gen';
import { patchSoftwareByIdVersionByVersionID } from '@/generated/api/requests/software/patchSoftwareByIdVersionByVersionID.gen';
import { postSoftware } from '@/generated/api/requests/software/postSoftware.gen';
import { postSoftwareByIdVersions } from '@/generated/api/requests/software/postSoftwareByIdVersions.gen';
import { parseResponse, requireAdmin } from '@/utils/api/page';

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

const versionSchema = z.object({
  id: z.uuid(),
  version: z.string(),
  lifecycle_status: z.enum(['CURRENT', 'OUTDATED', 'UNSUPPORTED', 'UNKNOWN']),
  release_date: z.string().nullable(),
  end_of_support: z.string().nullable()
});
export type ProductVersion = z.infer<typeof versionSchema>;

const AFFECTED = [
  'software',
  'balance',
  'findings',
  'dashboard',
  'audit',
  'contracts',
  'installations'
];

function refresh(queryClient: QueryClient) {
  return Promise.all(
    AFFECTED.map((resource) => queryClient.invalidateQueries({ queryKey: [resource] }))
  );
}

export async function saveProduct(
  auth: DemoAuth,
  queryClient: QueryClient,
  body: ProductInput & { archived?: boolean },
  id?: string
) {
  const session = requireAdmin(auth);
  try {
    const config = auth.requestConfig(session);
    const response = id
      ? await patchSoftwareById({ path: { id }, body, config })
      : await postSoftware({ body, config });
    return readProduct(response.data, 'ADMIN');
  } finally {
    if (auth.isCurrent(session.epoch)) await refresh(queryClient);
  }
}

export async function deleteProduct(auth: DemoAuth, queryClient: QueryClient, id: string) {
  const session = requireAdmin(auth);
  await deleteSoftwareById({ path: { id }, config: auth.requestConfig(session) });
  if (auth.isCurrent(session.epoch)) {
    queryClient.removeQueries({ queryKey: ['software', session.role, session.epoch, id] });
    await refresh(queryClient);
  }
}

export function versionsOptions(auth: DemoAuth, id: string) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['software', 'versions', session.role, session.epoch, id],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getSoftwareByIdVersions({ path: { id }, config: { signal } });
      return parseResponse(
        z.object({ items: z.array(versionSchema) }),
        response.data,
        'Сервер вернул некорректный список версий.'
      ).items;
    }
  });
}

export async function addVersion(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  body: {
    version: string;
    lifecycle_status: ProductVersion['lifecycle_status'];
    release_date?: string;
    end_of_support?: string;
  }
) {
  const session = requireAdmin(auth);
  try {
    await postSoftwareByIdVersions({ path: { id }, body, config: auth.requestConfig(session) });
  } finally {
    if (auth.isCurrent(session.epoch)) await refresh(queryClient);
  }
}

export async function updateVersion(
  auth: DemoAuth,
  queryClient: QueryClient,
  id: string,
  versionID: string,
  lifecycle_status: ProductVersion['lifecycle_status']
) {
  const session = requireAdmin(auth);
  try {
    await patchSoftwareByIdVersionByVersionID({
      path: { id, versionID },
      body: { lifecycle_status },
      config: auth.requestConfig(session)
    });
  } finally {
    if (auth.isCurrent(session.epoch)) await refresh(queryClient);
  }
}
