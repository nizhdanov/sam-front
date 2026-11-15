import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { ProductPage } from '@/features/software/product-page';

export const Route = createFileRoute('/_authenticated/software_/$productId')({
  beforeLoad: ({ params }) => {
    if (!z.uuid().safeParse(params.productId).success) throw notFound();
  },
  component: ProductPage
});
