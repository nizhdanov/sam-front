import { createFileRoute } from '@tanstack/react-router';
import { z } from 'zod';

import { AgreementsPage } from '@/features/agreements/agreement-page';

export const Route = createFileRoute('/_authenticated/_admin/agreements')({
  validateSearch: z.object({ page: z.coerce.number().int().positive().catch(1).default(1) }),
  component: AgreementsPage
});
