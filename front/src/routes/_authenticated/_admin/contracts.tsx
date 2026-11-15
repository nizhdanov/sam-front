import { createFileRoute } from '@tanstack/react-router';

import { ContractsPage } from '@/features/contracts/contracts-page';
import { contractsSearchSchema } from '@/features/contracts/model';

export const Route = createFileRoute('/_authenticated/_admin/contracts')({
  validateSearch: contractsSearchSchema,
  component: ContractsPage
});
