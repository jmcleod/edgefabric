// Reusable form dialog: wraps shadcn Dialog + react-hook-form + Zod validation.
// Renders a modal with a form, submit/cancel buttons, and loading state.

import { useEffect } from 'react';
import { useForm, type FieldValues, type DefaultValues, type Path } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import type { ZodSchema } from 'zod';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Loader2 } from 'lucide-react';
import { ResourceCombobox, type ComboboxOption } from '@/components/ui/ResourceCombobox';

export interface FieldConfig<T extends FieldValues> {
  name: Path<T>;
  label: string;
  type?: 'text' | 'number' | 'email' | 'password' | 'select' | 'textarea' | 'switch' | 'combobox';
  placeholder?: string;
  options?: { label: string; value: string }[];
  comboboxOptions?: ComboboxOption[];
  clearable?: boolean;
  description?: string;
  visibleWhen?: { field: Path<T>; value: unknown };
}

interface FormDialogProps<T extends FieldValues> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  schema: ZodSchema<T>;
  defaultValues?: DefaultValues<T>;
  fields: FieldConfig<T>[];
  onSubmit: (data: T) => void | Promise<void>;
  isSubmitting?: boolean;
  submitLabel?: string;
}

export function FormDialog<T extends FieldValues>({
  open,
  onOpenChange,
  title,
  description,
  schema,
  defaultValues,
  fields,
  onSubmit,
  isSubmitting = false,
  submitLabel = 'Save',
}: FormDialogProps<T>) {
  const form = useForm<T>({
    resolver: zodResolver(schema),
    defaultValues,
  });

  // Reset form when dialog opens with new default values
  useEffect(() => {
    if (open) {
      form.reset(defaultValues);
    }
  }, [open, defaultValues, form]);

  const handleSubmit = form.handleSubmit(async (data) => {
    await onSubmit(data);
  });

  // Watch all fields to support visibleWhen
  const watchedValues = form.watch();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {fields.map((field) => {
            // Conditional visibility
            if (field.visibleWhen) {
              const depValue = watchedValues[field.visibleWhen.field];
              if (depValue !== field.visibleWhen.value) {
                return null;
              }
            }

            const error = form.formState.errors[field.name];
            return (
              <div key={field.name} className="space-y-2">
                {field.type === 'switch' ? (
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <div className="space-y-0.5">
                      <Label htmlFor={field.name}>{field.label}</Label>
                      {field.description && (
                        <p className="text-xs text-muted-foreground">{field.description}</p>
                      )}
                    </div>
                    <Switch
                      id={field.name}
                      checked={!!form.watch(field.name)}
                      onCheckedChange={(checked) => form.setValue(field.name, checked as T[Path<T>])}
                    />
                  </div>
                ) : (
                  <>
                    <Label htmlFor={field.name}>{field.label}</Label>
                    {field.type === 'combobox' ? (
                      <ResourceCombobox
                        options={field.comboboxOptions || []}
                        value={(form.watch(field.name) as string) || ''}
                        onValueChange={(value) => form.setValue(field.name, value as T[Path<T>])}
                        placeholder={field.placeholder || `Select ${field.label.toLowerCase()}...`}
                        searchPlaceholder={`Search ${field.label.toLowerCase()}...`}
                        clearable={field.clearable}
                      />
                    ) : field.type === 'select' ? (
                      <Select
                        value={form.watch(field.name) as string}
                        onValueChange={(value) => form.setValue(field.name, value as T[Path<T>])}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder={field.placeholder || `Select ${field.label.toLowerCase()}`} />
                        </SelectTrigger>
                        <SelectContent>
                          {field.options?.map((opt) => (
                            <SelectItem key={opt.value} value={opt.value}>
                              {opt.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : field.type === 'textarea' ? (
                      <textarea
                        id={field.name}
                        className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                        placeholder={field.placeholder}
                        {...form.register(field.name)}
                      />
                    ) : (
                      <Input
                        id={field.name}
                        type={field.type || 'text'}
                        placeholder={field.placeholder}
                        {...form.register(field.name, {
                          valueAsNumber: field.type === 'number',
                        })}
                      />
                    )}
                    {field.description && field.type !== 'switch' && (
                      <p className="text-xs text-muted-foreground">{field.description}</p>
                    )}
                    {error && (
                      <p className="text-xs text-destructive">{error.message as string}</p>
                    )}
                  </>
                )}
              </div>
            );
          })}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
