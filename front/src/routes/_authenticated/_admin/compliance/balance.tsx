import { createFileRoute } from '@tanstack/react-router';

import { EmptyState } from '@/components/shared/states';

export const Route = createFileRoute('/_authenticated/_admin/compliance/balance')({
  component: () => (
    <EmptyState
      description='Сводные показатели дефицита и свободных мест доступны в обзоре. Детализация по продуктам появится после подключения данных баланса.'
      title={<h1>Баланс лицензий пока недоступен</h1>}
    />
  )
});
