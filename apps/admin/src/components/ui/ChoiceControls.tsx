"use client";

import { Checkbox, Select } from "@base-ui/react";
import { Check, ChevronDown } from "lucide-react";
import { ReactNode, useId, useState } from "react";
import { cn } from "@/lib/cn";

export interface ChoiceOption {
  value: string;
  label: string;
  description?: string;
}

export function BrandedSelect({
  label,
  value,
  options,
  onChange,
  placeholder = "Choose an option",
  disabled,
  compact = false,
}: {
  label: string;
  value: string;
  options: ChoiceOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  compact?: boolean;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <span id={`${id}-label`} className={cn("font-medium text-ink", compact ? "text-xs" : "text-sm")}>
        {label}
      </span>
      <Select.Root
        value={value || null}
        open={open}
        onOpenChange={setOpen}
        items={options}
        disabled={disabled}
        onValueChange={(next) => next !== null && onChange(next)}
      >
        <Select.Trigger
          aria-labelledby={`${id}-label`}
          className={cn(
            "group flex w-full items-center justify-between gap-3 rounded-xl border border-border-strong bg-surface-raised text-left text-ink shadow-sm outline-none transition-[border-color,box-shadow,background-color] hover:border-primary focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20 disabled:cursor-not-allowed disabled:opacity-50",
            compact ? "min-h-10 px-3 text-sm" : "min-h-11 px-3.5 text-sm",
          )}
        >
          <Select.Value placeholder={placeholder} className="min-w-0 truncate data-[placeholder]:text-ink-faint" />
          <Select.Icon className="shrink-0 text-primary transition-transform group-data-[popup-open]:rotate-180">
            <ChevronDown size={16} aria-hidden="true" />
          </Select.Icon>
        </Select.Trigger>
        <Select.Portal>
          <Select.Positioner sideOffset={8} alignItemWithTrigger={false} className="z-[100] outline-none">
            <Select.Popup className="min-w-[var(--anchor-width)] max-w-[min(24rem,calc(100vw-2rem))] origin-[var(--transform-origin)] overflow-hidden rounded-2xl border border-border bg-surface-raised p-1.5 text-ink shadow-xl outline-none data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0 transition-[transform,opacity]">
              <Select.List className="max-h-72 overflow-y-auto overscroll-contain">
                {options.map((option) => (
                  <Select.Item
                    key={option.value}
                    value={option.value}
                    onClick={() => { onChange(option.value); setOpen(false); }}
                    className="grid cursor-default grid-cols-[1fr_auto] items-center gap-3 rounded-xl px-3 py-2.5 outline-none transition-colors data-[highlighted]:bg-surface-sunken data-[selected]:text-primary"
                  >
                    <span className="min-w-0">
                      <Select.ItemText className="block truncate text-sm font-medium">{option.label}</Select.ItemText>
                      {option.description ? <span className="mt-0.5 block text-xs text-ink-muted">{option.description}</span> : null}
                    </span>
                    <Select.ItemIndicator className="text-primary"><Check size={16} strokeWidth={2.5} /></Select.ItemIndicator>
                  </Select.Item>
                ))}
              </Select.List>
            </Select.Popup>
          </Select.Positioner>
        </Select.Portal>
      </Select.Root>
    </div>
  );
}

export function BrandedCheckbox({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: ReactNode;
  description?: ReactNode;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <label className={cn("group flex cursor-pointer items-start gap-3 rounded-xl border bg-surface-raised p-4 transition-[border-color,background-color,box-shadow] hover:border-primary/60", checked ? "border-primary/50 bg-primary/5" : "border-border", disabled && "cursor-not-allowed opacity-50")}>
      <Checkbox.Root checked={checked} disabled={disabled} onCheckedChange={onChange} className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-md border-2 border-border-strong bg-surface outline-none transition-[border-color,background-color,box-shadow] group-hover:border-primary data-[checked]:border-primary data-[checked]:bg-primary focus-visible:ring-2 focus-visible:ring-primary/25 focus-visible:ring-offset-2 focus-visible:ring-offset-surface-raised">
        <Checkbox.Indicator className="text-on-primary"><Check size={13} strokeWidth={3} aria-hidden="true" /></Checkbox.Indicator>
      </Checkbox.Root>
      <span className="min-w-0">
        <span className="block text-sm font-medium text-ink">{label}</span>
        {description ? <span className="mt-1 block text-xs leading-relaxed text-ink-muted">{description}</span> : null}
      </span>
    </label>
  );
}
