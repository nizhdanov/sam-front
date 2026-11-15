import { createFileRoute } from '@tanstack/react-router';

import { ImportsPage } from '@/features/imports/imports-page';
import { importsSearchSchema } from '@/features/imports/model';

export const Route = createFileRoute('/_authenticated/_admin/imports')({
  validateSearch: importsSearchSchema,
  component: ImportsPage
});
