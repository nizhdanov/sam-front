import { createFileRoute } from '@tanstack/react-router';

import { AuditPage } from '@/features/audit/audit-page';
import { auditSearchSchema } from '@/features/audit/model';

export const Route = createFileRoute('/_authenticated/_admin/audit')({
  validateSearch: auditSearchSchema,
  component: AuditPage
});
