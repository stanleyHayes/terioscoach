"use client";
import { useRef, useState } from "react";
import { useAuth } from "@/lib/auth";
import { accountApi } from "@/lib/api";
import { DEFAULT_TIME_ZONE, DEFAULT_TIME_ZONE_LABEL } from "@/lib/timezones";
import { Button } from "./Button";
import { TimezonePicker } from "./TimezonePicker";

/** Account preference: a new account starts on US Eastern time, confirmed
 * with one tap; never silently save a browser-detected zone. */
export function TimezonePreference({
  onSaved,
}: {
  onSaved?: (zone: string) => void;
}) {
  const auth = useAuth();
  const [busy, setBusy] = useState(false);
  const saving = useRef(false);
  const [message, setMessage] = useState("");
  const current = auth.user?.timezone ?? "";
  async function save(zone: string) {
    if (!auth.session || saving.current) return;
    saving.current = true;
    setBusy(true);
    setMessage("");
    try {
      const { user } = await accountApi.updateTimezone(
        auth.session,
        auth.refreshCallbacks,
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
      <TimezonePicker
        label="Appointment timezone"
        value={current || DEFAULT_TIME_ZONE}
        onChange={(zone) => void save(zone)}
        disabled={busy || !auth.session}
      />
      {!current ? (
        <Button
          fullWidth
          loading={busy}
          disabled={!auth.session}
          onClick={() => void save(DEFAULT_TIME_ZONE)}
        >
          Continue with {DEFAULT_TIME_ZONE_LABEL}
        </Button>
      ) : null}
      <p role="status" className="text-sm text-ink-muted">
        {busy
          ? "Saving timezone…"
          : message ||
            (current
              ? "Appointments use this timezone on every device."
              : `${DEFAULT_TIME_ZONE_LABEL} is the practice default. Search above to choose another. Existing appointments will not move.`)}
      </p>
    </div>
  );
}
