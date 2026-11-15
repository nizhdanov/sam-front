import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { ImportPage } from '@/features/imports/import-page';

export const Route = createFileRoute('/_authenticated/_admin/imports_/$importId')({
  beforeLoad: ({ params }) => {
    if (!z.uuid().safeParse(params.importId).success) throw notFound();
  },
  component: ImportRoute
});

function ImportRoute() {
  const { importId } = Route.useParams();
  return <ImportPage key={importId} />;
}
