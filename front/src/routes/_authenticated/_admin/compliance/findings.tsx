import { createFileRoute } from '@tanstack/react-router';

import { FindingsPage } from '@/features/compliance/findings-page';
import { findingsSearchSchema } from '@/features/compliance/model';

export const Route = createFileRoute('/_authenticated/_admin/compliance/findings')({
  validateSearch: findingsSearchSchema,
  component: FindingsPage
});
