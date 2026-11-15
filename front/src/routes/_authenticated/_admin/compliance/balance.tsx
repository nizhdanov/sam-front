import { createFileRoute } from '@tanstack/react-router';

import { balanceSearchSchema } from '@/features/compliance/balance';
import { BalancePage } from '@/features/compliance/balance-page';

export const Route = createFileRoute('/_authenticated/_admin/compliance/balance')({
  validateSearch: balanceSearchSchema,
  component: BalancePage
});
