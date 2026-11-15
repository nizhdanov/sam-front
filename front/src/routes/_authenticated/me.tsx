import { createFileRoute } from '@tanstack/react-router';

import { myDeviceSearchSchema } from '@/features/compliance/model';
import { MyDevicePage } from '@/features/devices/my-device-page';

export const Route = createFileRoute('/_authenticated/me')({
  validateSearch: myDeviceSearchSchema,
  component: MyDevicePage
});
