import { createFileRoute } from '@tanstack/react-router';

import { AgreementsPage } from '@/features/agreements/agreement-page';

export const Route = createFileRoute('/_authenticated/_admin/agreements')({
  component: AgreementsPage
});
