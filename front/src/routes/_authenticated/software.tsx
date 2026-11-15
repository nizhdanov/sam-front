import { createFileRoute } from '@tanstack/react-router';

import { CatalogPage } from '@/features/software/catalog-page';
import { softwareSearchSchema } from '@/features/software/model';

export const Route = createFileRoute('/_authenticated/software')({
  validateSearch: softwareSearchSchema,
  component: CatalogPage
});
