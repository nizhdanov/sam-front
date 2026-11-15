import { Field, FieldLabel } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select';

export function FilterSelect({
  name,
  label,
  options,
  defaultValue = ''
}: {
  name: string;
  label: string;
  options: Record<string, string>;
  defaultValue?: string;
}) {
  const items = [
    { value: '', label: 'Все' },
    ...Object.entries(options).map(([value, text]) => ({ value, label: text }))
  ];
  return (
    <Field>
      <FieldLabel htmlFor={`filter-${name}`}>{label}</FieldLabel>
      <Select defaultValue={defaultValue} items={items} name={name}>
        <SelectTrigger className='w-full' id={`filter-${name}`}>
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
    </Field>
  );
}
