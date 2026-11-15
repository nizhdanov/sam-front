import type { DeviceCard } from '@/generated/api/types.gen';

import { formatDataValue } from '@/utils/api/data';

import { DEVICE_STATUSES, DEVICE_TYPES } from './model';

export function DeviceDetails({ device }: { device: DeviceCard }) {
  const details = [
    ['Серийный номер', device.serial_number],
    ['Тип устройства', device.device_type && DEVICE_TYPES[device.device_type]],
    ['Статус', device.status && DEVICE_STATUSES[device.status]],
    ['Имя компьютера', device.hostname],
    ['Операционная система', device.operating_system],
    ['Последний снимок', device.last_inventory_at],
    ['Установок', device.installations_count],
    ['Открытых нарушений', device.open_findings_count]
  ];
  return (
    <dl className='grid min-w-0 gap-4 sm:grid-cols-2'>
      {details.map(([label, value]) => (
        <div key={label} className='min-w-0'>
          <dt className='text-sm text-muted-foreground'>{label}</dt>
          <dd className='wrap-break-word'>{formatDataValue(value)}</dd>
        </div>
      ))}
    </dl>
  );
}
