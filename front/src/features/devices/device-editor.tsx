import type { z } from 'zod';

import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

import type { DeviceCard } from '@/generated/api/types.gen';

import { useDemoAuth } from '@/app/auth/context';
import { FormLeaveGuard } from '@/components/shared/form-leave-guard';
import { ErrorState, SuccessState } from '@/components/shared/states';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldGroup, FieldLabel, FieldSet } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';

import { DEVICE_STATUSES, DEVICE_TYPES, deviceFormSchema, getDeviceFormValues } from './model';
import { saveDevice } from './queries';

export function DeviceEditor({ device }: { device: DeviceCard & { id: string } }) {
  const { auth } = useDemoAuth();
  const session = auth.requireReady();
  const queryClient = useQueryClient();
  const [error, setError] = useState<unknown>();
  const [saved, setSaved] = useState(false);
  const form = useForm<
    z.input<typeof deviceFormSchema>,
    unknown,
    z.output<typeof deviceFormSchema>
  >({ resolver: zodResolver(deviceFormSchema), defaultValues: getDeviceFormValues(device) });
  const { isDirty, isSubmitting, errors } = form.formState;
  return (
    <form
      aria-label='Редактирование устройства'
      className='flex flex-col gap-4'
      onSubmit={form.handleSubmit(async (body) => {
        if (!auth.isCurrent(session.epoch)) return;
        setError(undefined);
        setSaved(false);
        try {
          const result = await saveDevice(auth, queryClient, device.id, body);
          if (!auth.isCurrent(session.epoch)) return;
          form.reset(getDeviceFormValues(result));
          setSaved(true);
        } catch (cause) {
          if (auth.isCurrent(session.epoch)) setError(cause);
        }
      })}
    >
      <h2 className='text-lg font-semibold'>Ручные поля</h2>
      <FormLeaveGuard when={() => auth.isCurrent(session.epoch) && (isDirty || isSubmitting)} />
      <FieldSet disabled={isSubmitting}>
        <FieldGroup className='grid gap-4 sm:grid-cols-2'>
          {(['device_type', 'status'] as const).map((name) => {
            const options =
              name === 'device_type'
                ? DEVICE_TYPES
                : device.status === 'ACTIVE'
                  ? { ACTIVE: DEVICE_STATUSES.ACTIVE }
                  : DEVICE_STATUSES;
            const items = Object.entries(options).map(([value, label]) => ({ value, label }));
            if (!device[name]) items.unshift({ value: '', label: 'Не указано' });
            return (
              <Controller
                key={name}
                render={({ field }) => (
                  <Field data-disabled={isSubmitting} data-invalid={!!errors[name]}>
                    <FieldLabel htmlFor={`edit-device-${name}`}>
                      {name === 'device_type' ? 'Тип устройства' : 'Статус устройства'}
                    </FieldLabel>
                    <Select
                      items={items}
                      name={field.name}
                      value={field.value}
                      onValueChange={(value) => field.onChange(value ?? '')}
                    >
                      <SelectTrigger
                        aria-invalid={!!errors[name]}
                        id={`edit-device-${name}`}
                        onBlur={field.onBlur}
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          {items.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FieldError errors={[errors[name]]} />
                  </Field>
                )}
                control={form.control}
                name={name}
              />
            );
          })}
          {(['hostname', 'responsible', 'location'] as const).map((name) => (
            <Field key={name} data-disabled={isSubmitting} data-invalid={!!errors[name]}>
              <FieldLabel htmlFor={`edit-device-${name}`}>
                {
                  {
                    hostname: 'Имя компьютера',
                    responsible: 'Ответственный',
                    location: 'Подразделение / кабинет'
                  }[name]
                }
              </FieldLabel>
              <Input
                {...form.register(name)}
                aria-invalid={!!errors[name]}
                id={`edit-device-${name}`}
              />
              <FieldError errors={[errors[name]]} />
            </Field>
          ))}
        </FieldGroup>
      </FieldSet>
      <div className='flex flex-wrap gap-2'>
        <Button disabled={isSubmitting || !isDirty} type='submit'>
          {isSubmitting && <Spinner data-icon='inline-start' />}Сохранить устройство
        </Button>
        <Button
          disabled={isSubmitting || !isDirty}
          variant='outline'
          onClick={() => {
            form.reset(getDeviceFormValues(device));
            setError(undefined);
            setSaved(false);
          }}
        >
          Сбросить изменения
        </Button>
      </div>
      {error !== undefined && <ErrorState error={error} />}
      {saved && !isDirty && <SuccessState>Устройство сохранено.</SuccessState>}
    </form>
  );
}
