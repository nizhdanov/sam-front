import { createFileRoute, notFound } from '@tanstack/react-router';
import { z } from 'zod';

import { DevicePage } from '@/features/devices/device-page';

export const Route = createFileRoute('/_authenticated/devices_/$deviceId')({
  beforeLoad: ({ params }) => {
    if (!z.uuid().safeParse(params.deviceId).success) throw notFound();
  },
  component: DevicePage
});
