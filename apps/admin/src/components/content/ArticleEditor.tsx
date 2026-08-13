"use client";

import { CircleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";
import { ImagePicker } from "@/components/content/ImagePicker";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { slugify, type Page, type Post } from "@/lib/content";

/**
 * The editor behind both pages and blog posts (ADM-07).
 *
 * The two differ only in the extra fields a post carries, so they share one
 * form rather than two that drift. What they also share is the rule that
 * matters most here: **saving never publishes**. Status is changed from the
 * list, by its own button, so an editor can revise a live page and take as
 * long over it as they like without anything going out half-written.
 */

export interface ArticleValues {
  slug: string;
  title: string;
  body: string;
  metaTitle: string;
  metaDescription: string;
  excerpt: string;
  coverImage: string;
  category: string;
  tags: string;
}

interface FieldErrors {
  slug?: string;
  title?: string;
  body?: string;
}

function initialValues(article: Page | Post | null): ArticleValues {
  const post = article as Post | null;
  return {
    slug: article?.slug ?? "",
    title: article?.title ?? "",
    body: article?.body ?? "",
    metaTitle: article?.metaTitle ?? "",
    metaDescription: article?.metaDescription ?? "",
    excerpt: post?.excerpt ?? "",
    coverImage: post?.coverImage ?? "",
    category: post?.category ?? "",
    tags: post?.tags?.join(", ") ?? "",
  };
}

export function ArticleEditor({
  kind,
  article,
  onClose,
  onSubmit,
}: {
  kind: "page" | "post";
  /** null → create; an article → edit. */
  article: Page | Post | null;
  onClose: () => void;
  /** The parent performs the call and throws on failure. */
  onSubmit: (values: ArticleValues) => Promise<void>;
}) {
  const editing = article !== null;
  const initial = initialValues(article);

  const [values, setValues] = useState(initial);
  // An untouched slug tracks the title; once it has been edited by hand it
  // stops moving, because an established URL is not something to rewrite
  // just because a headline was reworded.
  const [slugTouched, setSlugTouched] = useState(editing);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty = (Object.keys(values) as (keyof ArticleValues)[]).some(
    (key) => values[key] !== initial[key],
  );

  function update<K extends keyof ArticleValues>(key: K, value: ArticleValues[K]) {
    setValues((current) => ({ ...current, [key]: value }));
    setFieldErrors((errors) => ({ ...errors, [key]: undefined }));
  }

  function handleTitle(title: string) {
    setValues((current) => ({
      ...current,
      title,
      slug: slugTouched ? current.slug : slugify(title),
    }));
    setFieldErrors((errors) => ({ ...errors, title: undefined, slug: undefined }));
  }

  function validate(): boolean {
    const errors: FieldErrors = {};
    if (!values.title.trim()) {
      errors.title = "Give it a title";
    }
    if (!values.slug.trim()) {
      errors.slug = "A web address is required";
    } else if (values.slug.trim() !== slugify(values.slug)) {
      errors.slug = "Use lowercase letters, numbers and hyphens only";
    }
    if (!values.body.trim()) {
      errors.body = "There's nothing to publish yet";
    }
    setFieldErrors(errors);
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    if (!validate()) return;

    setSubmitting(true);
    try {
      await onSubmit(values);
      onClose();
    } catch (error) {
      setFormError(
        error instanceof ApiError ? error.message : "Something went wrong. Try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  const noun = kind === "page" ? "page" : "post";
  const publicPath = kind === "page" ? `/${values.slug}` : `/blog/${values.slug}`;

  return (
    <Modal
      open
      onClose={onClose}
      title={editing ? `Edit ${noun}` : `New ${noun}`}
      description={
        editing && article?.status === "published"
          ? "This is live. Your changes go out as soon as you save."
          : "Saving keeps it as a draft — publish it when you're ready."
      }
      size="form"
      dirty={dirty}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="article-form" loading={submitting}>
            {editing ? "Save changes" : `Create ${noun}`}
          </Button>
        </>
      }
    >
      <form id="article-form" noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
        {formError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
            {formError}
          </div>
        ) : null}

        <TextInput
          label="Title"
          required
          data-autofocus
          value={values.title}
          error={fieldErrors.title}
          placeholder={kind === "page" ? "About the practice" : "Five ways to rest properly"}
          onChange={(event) => handleTitle(event.target.value)}
        />

        <TextInput
          label="Web address"
          required
          value={values.slug}
          error={fieldErrors.slug}
          hint={values.slug ? `Visitors will find this at ${publicPath}` : undefined}
          onChange={(event) => {
            setSlugTouched(true);
            update("slug", event.target.value);
          }}
        />

        {kind === "post" ? (
          <>
            <TextArea
              label="Excerpt"
              rows={2}
              value={values.excerpt}
              hint="The line that appears under the title on the blog index."
              onChange={(event) => update("excerpt", event.target.value)}
            />
            <ImagePicker
              value={values.coverImage}
              disabled={submitting}
              onChange={(url) => update("coverImage", url)}
            />
          </>
        ) : null}

        <TextArea
          label="Body"
          required
          rows={12}
          value={values.body}
          error={fieldErrors.body}
          hint="Plain text. A blank line starts a new paragraph."
          onChange={(event) => update("body", event.target.value)}
        />

        {kind === "post" ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <TextInput
              label="Category"
              value={values.category}
              placeholder="Wellbeing"
              onChange={(event) => update("category", event.target.value)}
            />
            <TextInput
              label="Tags"
              value={values.tags}
              hint="Separated by commas."
              placeholder="rest, sleep"
              onChange={(event) => update("tags", event.target.value)}
            />
          </div>
        ) : null}

        <fieldset className="flex flex-col gap-4 rounded-lg border border-border p-4">
          <legend className="px-1.5 text-[13px] font-medium text-ink-muted">
            Search engines
          </legend>
          <TextInput
            label="Meta title"
            value={values.metaTitle}
            hint="Leave blank to use the title above."
            onChange={(event) => update("metaTitle", event.target.value)}
          />
          <TextArea
            label="Meta description"
            rows={2}
            value={values.metaDescription}
            hint="Around 155 characters is what a search result shows."
            onChange={(event) => update("metaDescription", event.target.value)}
          />
        </fieldset>
      </form>
    </Modal>
  );
}

/** Turns the form's flat strings into the create/patch body the API takes. */
export function toArticleBody(kind: "page" | "post", values: ArticleValues) {
  const base = {
    slug: values.slug.trim(),
    title: values.title.trim(),
    body: values.body.trim(),
    metaTitle: values.metaTitle.trim(),
    metaDescription: values.metaDescription.trim(),
  };
  if (kind === "page") return base;
  return {
    ...base,
    excerpt: values.excerpt.trim(),
    coverImage: values.coverImage.trim(),
    category: values.category.trim(),
    tags: values.tags
      .split(",")
      .map((tag) => tag.trim())
      .filter(Boolean),
  };
}
