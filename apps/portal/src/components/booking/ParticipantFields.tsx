"use client";
import type { Participant } from "@/lib/bookings";
import { TextInput } from "@/components/ui/TextInput";
import { BrandedCheckbox } from "@/components/ui/ChoiceControls";
export function ParticipantFields({
  value,
  onChange,
  consentBody,
  signature,
  onSignature,
  acknowledged,
  onAcknowledged,
}: {
  value: Participant;
  onChange: (value: Participant) => void;
  consentBody?: string;
  signature?: string;
  onSignature?: (value: string) => void;
  acknowledged?: boolean;
  onAcknowledged?: (value: boolean) => void;
}) {
  return (
    <fieldset className="space-y-4 rounded-2xl border border-border p-5">
      <legend className="px-2 font-semibold">
        Who is this appointment for?
      </legend>
      <TextInput
        label="Participant full name"
        value={value.name}
        required
        maxLength={120}
        onChange={(e) =>
          onChange({ ...value, name: e.target.value, accurate: false })
        }
      />
      <BrandedCheckbox
        label="This appointment is for someone under 18."
        description="Age on the appointment date. A parent or guardian must sign the required documents before the session."
        checked={value.under18}
        onChange={(under18) => onChange({ ...value, under18, accurate: false })}
      />
      {value.under18 && (
        <>
          <TextInput
            label="Guardian full name"
            required
            maxLength={120}
            value={value.guardianName ?? ""}
            onChange={(e) =>
              onChange({ ...value, guardianName: e.target.value })
            }
          />
          <TextInput
            label="Relationship to participant"
            required
            maxLength={120}
            value={value.guardianRelationship ?? ""}
            onChange={(e) =>
              onChange({ ...value, guardianRelationship: e.target.value })
            }
          />
          <TextInput
            label="Guardian contact email"
            type="email"
            required
            value={value.guardianEmail ?? ""}
            onChange={(e) =>
              onChange({ ...value, guardianEmail: e.target.value })
            }
          />
          {onSignature && (
            <>
              <p className="whitespace-pre-wrap text-sm leading-relaxed text-ink">
                {consentBody || "Loading guardian consent wording…"}
              </p>
              <TextInput
                label="Parent / guardian electronic signature"
                maxLength={120}
                value={signature ?? ""}
                onChange={(e) => onSignature(e.target.value)}
                hint="The parent or guardian enters their own name. You may sign now or complete consent before the session."
              />
              <BrandedCheckbox
                label="I am the parent or guardian named above. I have read the consent wording and intend my typed name to be my electronic signature."
                checked={acknowledged ?? false}
                onChange={(value) => onAcknowledged?.(value)}
              />
            </>
          )}
          <p className="text-sm text-ink-muted">
            This contact declaration does not verify the guardian’s identity.
          </p>
        </>
      )}
      <BrandedCheckbox
        label="I confirm these participant details are accurate for the appointment date."
        checked={value.accurate}
        onChange={(accurate) => onChange({ ...value, accurate })}
      />
    </fieldset>
  );
}
