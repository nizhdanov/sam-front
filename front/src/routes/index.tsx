import { createFileRoute, redirect } from '@tanstack/react-router';

import { getLanding } from '@/app/auth/demo-accounts';

export const Route = createFileRoute('/')({
  beforeLoad: ({ context }) => {
    throw redirect({ to: getLanding(context.auth.requireReady().role), replace: true });
  }
});
