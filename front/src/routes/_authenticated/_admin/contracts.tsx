import { createFileRoute } from '@tanstack/react-router';

import { SectionPlaceholder } from '@/components/shared/states';

export const Route = createFileRoute('/_authenticated/_admin/contracts')({
  component: () => <SectionPlaceholder title='Договоры' />
});
