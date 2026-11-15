import { createFileRoute } from '@tanstack/react-router';

import { MatchingPage } from '@/features/matching/matching-page';
import { matchingSearchSchema } from '@/features/matching/model';

export const Route = createFileRoute('/_authenticated/_admin/matching')({
  validateSearch: matchingSearchSchema,
  component: MatchingPage
});
