"use client";

import { useState } from "react";
import { ImagePicker } from "./ImagePicker";
import { BrandedSelect } from "@/components/ui/ChoiceControls";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { pagesApi, type Page } from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";
import {
  contentSlug,
  legacySiteValues,
  parseSiteValues,
  validContentUrl,
  websiteContent,
  type SiteValues,
} from "../../../../../shared/site-content";

export function WebsiteEditor() {
  const pages = useResource(pagesApi.list);
  const [scope, setScope] = useState("home");
  const [dirty, setDirty] = useState(false);
  const [requestedScope, setRequestedScope] = useState<string | null>(null);
  return (
    <div className="flex flex-col gap-5">
      <div>
        <h2 className="text-xl font-semibold text-ink">Website sections</h2>
        <p className="mt-2 text-sm text-ink-muted">
          Edit the text, links and images on each page. Services, blog posts,
          FAQs and reviews use their own editors. Changes appear when you save
          and publish.
        </p>
      </div>
      <div className="max-w-sm">
        <BrandedSelect
          label="Page or shared section"
          value={scope}
          onChange={(value) =>
            dirty ? setRequestedScope(value) : setScope(value)
          }
          options={Object.entries(websiteContent).map(([value, item]) => ({
            value,
            label: item.label,
          }))}
        />
      </div>
      {pages.error ? (
        <div role="alert">
          {pages.error} <Button onClick={pages.refresh}>Try again</Button>
        </div>
      ) : !pages.data ? (
        <p role="status">Loading website content…</p>
      ) : (
        <SectionEditor
          key={scope}
          scope={scope}
          pages={pages.data}
          onDirty={setDirty}
          onSaved={(saved) =>
            pages.set((current) => [
              ...(current ?? []).filter((page) => page.id !== saved.id),
              saved,
            ])
          }
        />
      )}
      <Modal
        open={requestedScope !== null}
        onClose={() => setRequestedScope(null)}
        title="Leave unsaved changes?"
        footer={
          <>
            <Button variant="ghost" onClick={() => setRequestedScope(null)}>
              Keep editing
            </Button>
            <Button
              onClick={() => {
                if (requestedScope) setScope(requestedScope);
                setRequestedScope(null);
                setDirty(false);
              }}
            >
              Discard changes
            </Button>
          </>
        }
      >
        <p className="text-sm text-ink-muted">
          Your changes to this section have not been published.
        </p>
      </Modal>
    </div>
  );
}

function SectionEditor({
  scope,
  pages,
  onDirty,
  onSaved,
}: {
  scope: string;
  pages: Page[];
  onDirty: (dirty: boolean) => void;
  onSaved: (page: Page) => void;
}) {
  const definition = websiteContent[scope];
  const [record, setRecord] = useState(() =>
    pages.find((page) => page.slug === contentSlug(scope)),
  );
  const [values, setValues] = useState<SiteValues>(() => ({
    ...Object.fromEntries(
      definition.fields.map((field) => [field.key, field.value]),
    ),
    ...legacySiteValues(
      scope,
      pages.find((page) => page.slug === scope && page.status === "published"),
    ),
    ...parseSiteValues(record?.body ?? ""),
  }));
  const [notice, setNotice] = useState("");
  const [validation, setValidation] = useState("");
  const [dirty, setDirty] = useState(false);
  const action = useAction();
  const groups = [...new Set(definition.fields.map((field) => field.group))];
  const setValue = (key: string, value: string) => {
    setValues((current) => ({ ...current, [key]: value }));
    setDirty(true);
    onDirty(true);
    setNotice("");
  };

  async function save() {
    setValidation("");
    setNotice("");
    const invalid = definition.fields.find(
      (field) =>
        (field.type === "image" || field.type === "link") &&
        !validContentUrl(values[field.key], field.type === "image"),
    );
    if (invalid) {
      setValidation(
        `Check ${invalid.label}: use a website path or secure URL${invalid.type === "link" ? ", email or phone link" : ""}.`,
      );
      return;
    }
    const saved = await action.run("save", async (session, callbacks) => {
      const draft = {
        slug: contentSlug(scope),
        title: `${definition.label} website sections`,
        body: JSON.stringify(values),
      };
      const updated = record
        ? await pagesApi.update(session, callbacks, record.id, draft)
        : await pagesApi.create(session, callbacks, draft);
      // Retain the created ID if publishing fails so retry never creates a duplicate.
      setRecord(updated);
      onSaved(updated);
      return updated.status === "published"
        ? updated
        : await pagesApi.setPublished(session, callbacks, updated.id, true);
    });
    if (saved) {
      setRecord(saved);
      onSaved(saved);
      setDirty(false);
      onDirty(false);
      setNotice(`${definition.label} changes published.`);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="sticky top-0 z-20 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-surface-raised p-4 shadow-sm">
        <p className="text-sm text-ink-muted">
          {definition.fields.length} editable fields
          {dirty ? " · Unsaved changes" : ""}
        </p>
        <Button disabled={Boolean(action.pending)} onClick={() => void save()}>
          {action.pending ? "Publishing…" : "Save and publish"}
        </Button>
        {notice ? (
          <p role="status" className="w-full text-sm text-primary">
            {notice}
          </p>
        ) : null}
        {validation || action.error ? (
          <p role="alert" className="w-full text-sm text-danger-ink">
            {validation || action.error}
          </p>
        ) : null}
      </div>
      <fieldset
        disabled={Boolean(action.pending)}
        className="flex flex-col gap-4"
      >
        {groups.map((group, index) => (
          <details
            key={group}
            open={index === 0}
            className="rounded-2xl border border-border bg-surface-raised p-5"
          >
            <summary className="cursor-pointer text-base font-semibold capitalize text-ink">
              {group}
            </summary>
            <div className="mt-5 grid gap-6">
              {definition.fields
                .filter((field) => field.group === group)
                .map((field) => (
                  <div key={field.key} className="min-w-0">
                    {field.type === "image" ? (
                      <>
                        <p className="mb-2 text-sm font-medium">
                          {field.label}
                        </p>
                        <ImagePicker
                          label={`${definition.label}: ${field.label}`}
                          value={values[field.key]}
                          onChange={(value) => setValue(field.key, value)}
                        />
                      </>
                    ) : (
                      <div className="flex flex-col gap-2 text-sm font-medium text-ink">
                        <label htmlFor={`${scope}-${field.key}`}>
                          {field.label}
                        </label>
                        {field.type === "link" ? (
                          <input
                            id={`${scope}-${field.key}`}
                            value={values[field.key]}
                            onChange={(event) =>
                              setValue(field.key, event.target.value)
                            }
                            className="w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm"
                          />
                        ) : (
                          <textarea
                            id={`${scope}-${field.key}`}
                            rows={values[field.key].length > 100 ? 4 : 2}
                            value={values[field.key]}
                            onChange={(event) =>
                              setValue(field.key, event.target.value)
                            }
                            className="w-full resize-y rounded-xl border border-border bg-surface px-3 py-2 text-sm leading-relaxed"
                          />
                        )}
                      </div>
                    )}
                  </div>
                ))}
            </div>
          </details>
        ))}
      </fieldset>
    </div>
  );
}
