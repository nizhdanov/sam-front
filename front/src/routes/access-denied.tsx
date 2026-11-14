import { createFileRoute } from '@tanstack/react-router';

import { AccessDenied } from '@/components/shared/route-states';

export const Route = createFileRoute('/access-denied')({ component: AccessDenied });
