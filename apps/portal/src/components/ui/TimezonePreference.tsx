"use client";
import { useMemo, useRef, useState } from "react";
import { useAuth } from "@/lib/auth";
import { accountApi } from "@/lib/api";
import { BrandedSelect } from "./ChoiceControls";
import { TextInput } from "./TextInput";

/** Account preference: never silently save a browser-detected zone. */
export function TimezonePreference({
  onSaved,
}: {
  onSaved?: (zone: string) => void;
}) {
  const auth = useAuth();
  const [search, setSearch] = useState("");
  const [busy, setBusy] = useState(false);
  const saving = useRef(false);
  const [message, setMessage] = useState("");
  const options = useMemo(() => {
    const zones = Array.from(
      new Set([
        "UTC",
        auth.user?.timezone ?? "UTC",
        ...Intl.supportedValuesOf("timeZone"),
      ]),
    );
    return zones
      .filter(
        (zone) =>
          zone === auth.user?.timezone ||
          zone
            .toLowerCase()
            .replaceAll("_", " ")
            .includes(search.toLowerCase()),
      )
      .map((zone) => ({ value: zone, label: zone.replaceAll("_", " ") }));
  }, [search, auth.user?.timezone]);
  async function save(zone: string) {
    if (!auth.session || saving.current) return;
    saving.current = true;
    setBusy(true);
    setMessage("");
    try {
      const { user } = await accountApi.updateTimezone(
        auth.session,
        { onTokensRefreshed: auth.onTokensRefreshed },
        zone,
      );
      auth.setUserProfile(user);
      onSaved?.(zone);
      setMessage("Timezone saved across your account.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Timezone could not be saved. Try again.",
      );
    } finally {
      saving.current = false;
      setBusy(false);
    }
  }
  return (
    <div className="space-y-3">
      <TextInput
        label="Search timezones"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
        placeholder="City or region"
      />
      <BrandedSelect
        label="Appointment timezone"
        value={auth.user?.timezone ?? ""}
        options={options}
        onChange={(zone) => void save(zone)}
        disabled={busy || !auth.session}
        placeholder="Choose and confirm your timezone"
      />
      <p role="status" className="text-sm text-ink-muted">
        {busy
          ? "Saving timezone…"
          : message ||
            (auth.user?.timezone
              ? "Appointments use this timezone on every device."
              : "Choose your timezone before scheduling. Existing appointments will not move.")}
      </p>
    </div>
  );
}
