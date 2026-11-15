import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { DemoAuth } from '@/app/auth/session';

import { getAudit } from '@/generated/api/requests/audit/getAudit.gen';
import { pageSchema, parseResponse, requireAdmin } from '@/utils/api/page';

const optionalText = z
  .string()
  .trim()
  .transform((value) => value || undefined)
  .optional();
const optionalDate = z.union([z.iso.date(), z.literal('').transform(() => undefined)]).optional();

export const auditSearchSchema = z.object({
  actor: optionalText,
  entity: optionalText,
  from: optionalDate,
  to: optionalDate,
  page: z.coerce.number().int().positive().default(1)
});
export type AuditSearch = z.infer<typeof auditSearchSchema>;

export const AUDIT_ACTIONS: Record<string, string> = {
  CREATE: 'Создание',
  UPDATE: 'Изменение',
  DELETE: 'Удаление',
  APPROVE: 'Подтверждение',
  REJECT: 'Отклонение'
};

const entrySchema = z.object({
  id: z.number(),
  created_at: z.string(),
  actor: z.string(),
  entity: z.string(),
  entity_id: z.string().optional(),
  action: z.string(),
  summary: z.string(),
  changes: z.record(z.string(), z.unknown()).optional()
});
export type AuditEntry = z.infer<typeof entrySchema>;

function dayStart(date: string) {
  return new Date(`${date}T00:00:00`).toISOString();
}

function dayEnd(date: string) {
  return new Date(`${date}T23:59:59.999`).toISOString();
}

export function auditOptions(auth: DemoAuth, search: AuditSearch) {
  const session = requireAdmin(auth);
  return queryOptions({
    queryKey: ['audit', session.role, session.epoch, search],
    queryFn: async ({ signal }) => {
      auth.requestConfig(session);
      const response = await getAudit({
        query: {
          page: search.page,
          limit: 50,
          actor: search.actor,
          entity: search.entity,
          from: search.from && dayStart(search.from),
          to: search.to && dayEnd(search.to)
        },
        config: { signal }
      });
      return parseResponse(
        pageSchema(entrySchema),
        response.data,
        'Сервер вернул некорректный журнал.'
      );
    }
  });
}
