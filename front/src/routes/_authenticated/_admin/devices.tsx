import { createFileRoute } from '@tanstack/react-router';

import { DevicesPage } from '@/features/devices/devices-page';
import { devicesSearchSchema } from '@/features/devices/model';

export const Route = createFileRoute('/_authenticated/_admin/devices')({
  validateSearch: devicesSearchSchema,
  component: DevicesPage
});
