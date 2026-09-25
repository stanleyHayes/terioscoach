"use client";

import { Combobox } from "@base-ui/react";
import { Check, ChevronDown, Search } from "lucide-react";
import { useId, useMemo } from "react";
import {
  matchesTimeZone,
  timeZoneGroups,
  type TimeZoneGroup,
  type TimeZoneOption,
} from "@/lib/timezones";

/**
 * One field for choosing a time zone: it opens a panel whose search box
 * filters the list as you type, United States zones first.
 */
export function TimezonePicker({
  label,
  value,
  onChange,
  disabled,
  placeholder = "Choose a time zone",
}: {
  label: string;
  value: string;
  onChange: (zone: string) => void;
  disabled?: boolean;
  placeholder?: string;
}) {
  const id = useId();
  const groups = useMemo(() => timeZoneGroups(value || undefined), [value]);
  const selected = useMemo(
    () => groups.flatMap((g) => g.items).find((o) => o.value === value) ?? null,
    [groups, value],
  );
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <span id={`${id}-label`} className="text-sm font-medium text-ink">
        {label}
      </span>
      <Combobox.Root
        items={groups}
        value={selected}
        onValueChange={(next: TimeZoneOption | null) => {
          if (next && next.value !== value) onChange(next.value);
        }}
        itemToStringLabel={(item: TimeZoneOption) => item.display}
        isItemEqualToValue={(a: TimeZoneOption, b: TimeZoneOption) => a.value === b.value}
        filter={(item: TimeZoneOption, query: string) => matchesTimeZone(item, query)}
        disabled={disabled}
      >
        <Combobox.Trigger
          aria-labelledby={`${id}-label`}
          className="group flex min-h-11 w-full items-center justify-between gap-3 rounded-xl border border-border-strong bg-surface-raised px-3.5 text-left text-sm text-ink shadow-sm outline-none transition-[border-color,box-shadow,background-color] hover:border-primary focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="min-w-0 truncate">
            {selected ? selected.display : <span className="text-ink-faint">{placeholder}</span>}
          </span>
          <ChevronDown
            size={16}
            aria-hidden="true"
            className="shrink-0 text-primary transition-transform group-data-[popup-open]:rotate-180"
          />
        </Combobox.Trigger>
        <Combobox.Portal>
          <Combobox.Positioner sideOffset={8} className="z-[100] outline-none">
            <Combobox.Popup
              aria-label={label}
              className="flex max-h-[min(26rem,var(--available-height))] w-[var(--anchor-width)] min-w-[min(20rem,calc(100vw-2rem))] max-w-[calc(100vw-2rem)] origin-[var(--transform-origin)] flex-col overflow-hidden rounded-2xl border border-border bg-surface-raised text-ink shadow-xl outline-none transition-[transform,opacity] data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0"
            >
              <div className="relative border-b border-border p-2">
                <Search
                  size={16}
                  aria-hidden="true"
                  className="pointer-events-none absolute top-1/2 left-5 -translate-y-1/2 text-ink-faint"
                />
                <Combobox.Input
                  aria-label="Search time zones"
                  placeholder="Search city, region or time zone"
                  className="h-10 w-full rounded-lg border border-border bg-surface pr-3 pl-9 text-base text-ink outline-none placeholder:text-ink-faint focus:border-primary focus:ring-2 focus:ring-primary/20 sm:text-sm"
                />
              </div>
              <Combobox.Empty className="px-4 py-6 text-center text-sm text-ink-muted empty:hidden">
                No time zone matches that search.
              </Combobox.Empty>
              <Combobox.List className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-1.5 empty:hidden">
                {(group: TimeZoneGroup) => (
                  <Combobox.Group key={group.label} items={group.items} className="pb-1">
                    <Combobox.GroupLabel className="px-3 pt-2 pb-1 text-[11px] font-semibold tracking-[0.08em] text-ink-faint uppercase">
                      {group.label}
                    </Combobox.GroupLabel>
                    <Combobox.Collection>
                      {(item: TimeZoneOption) => (
                        <Combobox.Item
                          key={item.value}
                          value={item}
                          className="grid cursor-default grid-cols-[1fr_auto] items-center gap-3 rounded-xl px-3 py-2.5 outline-none transition-colors data-[highlighted]:bg-surface-sunken data-[selected]:text-primary"
                        >
                          <span className="min-w-0">
                            <span className="block truncate text-sm font-medium">{item.label}</span>
                            <span className="mt-0.5 block truncate text-xs text-ink-muted">{item.detail}</span>
                          </span>
                          <Combobox.ItemIndicator className="text-primary">
                            <Check size={16} strokeWidth={2.5} />
                          </Combobox.ItemIndicator>
                        </Combobox.Item>
                      )}
                    </Combobox.Collection>
                  </Combobox.Group>
                )}
              </Combobox.List>
            </Combobox.Popup>
          </Combobox.Positioner>
        </Combobox.Portal>
      </Combobox.Root>
    </div>
  );
}
