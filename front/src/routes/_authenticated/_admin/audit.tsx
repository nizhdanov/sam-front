import { createFileRoute } from '@tanstack/react-router';

import { EmptyState } from '@/components/shared/states';

export const Route = createFileRoute('/_authenticated/_admin/audit')({
  component: () => (
    <EmptyState
      description='История операций появится после подключения серверного журнала.'
      title={<h1>Журнал изменений пока недоступен</h1>}
    />
  )
});
