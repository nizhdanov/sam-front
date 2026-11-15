import { createFileRoute } from '@tanstack/react-router';

import { InstallationsPage } from '@/features/installations/installations-page';
import { installationsSearchSchema } from '@/features/installations/model';

export const Route = createFileRoute('/_authenticated/_admin/installations')({
  validateSearch: installationsSearchSchema,
  component: InstallationsPage
});
