import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { AgreementPage } from '@/features/agreements/agreement-page';

export const Route = createFileRoute('/_authenticated/_admin/agreements_/$agreementId')({
  beforeLoad: ({ params }) => {
    if (!z.uuid().safeParse(params.agreementId).success) throw notFound();
  },
  component: AgreementPage
});
