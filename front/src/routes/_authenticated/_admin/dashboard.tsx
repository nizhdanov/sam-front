import { createFileRoute } from '@tanstack/react-router';

import { DashboardPage } from '@/features/compliance/dashboard-page';

export const Route = createFileRoute('/_authenticated/_admin/dashboard')({
  component: DashboardPage
});
