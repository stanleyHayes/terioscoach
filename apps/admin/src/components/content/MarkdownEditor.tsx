"use client";

import { useEffect, useId, useRef, useState } from "react";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { Markdown } from "@tiptap/markdown";
import Image from "@tiptap/extension-image";
import { TableKit } from "@tiptap/extension-table";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import {
  Bold,
  Italic,
  Strikethrough,
  List,
  ListOrdered,
  ListTodo,
  Quote,
  Code,
  Code2,
  Link2,
  Unlink,
  ImagePlus,
  Table2,
  Minus,
  Undo2,
  Redo2,
  RemoveFormatting,
} from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ImagePicker } from "./ImagePicker";
import { BrandedSelect } from "@/components/ui/ChoiceControls";
import { cn } from "@/lib/cn";

export function MarkdownEditor({
  value,
  onChange,
  error,
}: {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  const id = useId();
  const [mode, setMode] = useState<"rich" | "markdown" | "preview">("rich");
  const [insert, setInsert] = useState<"link" | "image" | null>(null);
  const [url, setUrl] = useState("");
  const [linkError, setLinkError] = useState("");
  const latest = useRef(value);
  const change = useRef(onChange);
  useEffect(() => {
    change.current = onChange;
  }, [onChange]);
  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit.configure({ underline: false, link: { openOnClick: false } }),
      Markdown,
      Image,
      TableKit,
      TaskList,
      TaskItem.configure({ nested: true }),
    ],
    content: value,
    contentType: "markdown",
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-label": "Body",
        "aria-multiline": "true",
        class: "tiptap terios-markdown min-h-[26rem] px-5 py-5 outline-none",
      },
    },
    onUpdate: ({ editor }) => {
      const next = editor.getMarkdown();
      latest.current = next;
      change.current(next);
    },
  });
  useEditorState({ editor, selector: ({ editor }) => editor?.state });
  useEffect(() => {
    if (editor && value !== latest.current) {
      latest.current = value;
      editor.commands.setContent(value, {
        contentType: "markdown",
        emitUpdate: false,
      });
    }
  }, [editor, value]);
  useEffect(() => {
    editor?.view.dom.setAttribute("aria-invalid", String(Boolean(error)));
  }, [editor, error]);

  const tools = editor
    ? [
        {
          label: "Bold",
          icon: Bold,
          active: editor.isActive("bold"),
          run: () => editor.chain().focus().toggleBold().run(),
        },
        {
          label: "Italic",
          icon: Italic,
          active: editor.isActive("italic"),
          run: () => editor.chain().focus().toggleItalic().run(),
        },
        {
          label: "Strikethrough",
          icon: Strikethrough,
          active: editor.isActive("strike"),
          run: () => editor.chain().focus().toggleStrike().run(),
        },
        {
          label: "Bulleted list",
          icon: List,
          active: editor.isActive("bulletList"),
          run: () => editor.chain().focus().toggleBulletList().run(),
        },
        {
          label: "Numbered list",
          icon: ListOrdered,
          active: editor.isActive("orderedList"),
          run: () => editor.chain().focus().toggleOrderedList().run(),
        },
        {
          label: "Task list",
          icon: ListTodo,
          active: editor.isActive("taskList"),
          run: () => editor.chain().focus().toggleTaskList().run(),
        },
        {
          label: "Quote",
          icon: Quote,
          active: editor.isActive("blockquote"),
          run: () => editor.chain().focus().toggleBlockquote().run(),
        },
        {
          label: "Inline code",
          icon: Code,
          active: editor.isActive("code"),
          run: () => editor.chain().focus().toggleCode().run(),
        },
        {
          label: "Code block",
          icon: Code2,
          active: editor.isActive("codeBlock"),
          run: () => editor.chain().focus().toggleCodeBlock().run(),
        },
        {
          label: "Link",
          icon: Link2,
          active: editor.isActive("link"),
          run: () => {
            setUrl(editor.getAttributes("link").href || "");
            setInsert(insert === "link" ? null : "link");
          },
        },
        {
          label: "Remove link",
          icon: Unlink,
          run: () => editor.chain().focus().unsetLink().run(),
          disabled: !editor.isActive("link"),
        },
        {
          label: "Insert image",
          icon: ImagePlus,
          run: () => {
            setUrl("");
            setInsert(insert === "image" ? null : "image");
          },
        },
        {
          label: "Insert table",
          icon: Table2,
          run: () =>
            editor
              .chain()
              .focus()
              .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
              .run(),
        },
        {
          label: "Divider",
          icon: Minus,
          run: () => editor.chain().focus().setHorizontalRule().run(),
        },
        {
          label: "Clear formatting",
          icon: RemoveFormatting,
          run: () => editor.chain().focus().unsetAllMarks().clearNodes().run(),
        },
        {
          label: "Undo",
          icon: Undo2,
          run: () => editor.chain().focus().undo().run(),
          disabled: !editor.can().undo(),
        },
        {
          label: "Redo",
          icon: Redo2,
          run: () => editor.chain().focus().redo().run(),
          disabled: !editor.can().redo(),
        },
      ]
    : [];

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p id={id} className="text-sm font-medium text-ink">
            Body <span className="text-accent">*</span>
          </p>
          <p className="mt-1 text-xs text-ink-faint">
            Write visually, edit Markdown, or preview your page.
          </p>
        </div>
        <div
          role="tablist"
          aria-label="Body editor mode"
          className="flex rounded-xl border border-border bg-surface-sunken p-1"
        >
          {(["rich", "markdown", "preview"] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              role="tab"
              aria-selected={mode === tab}
              onClick={() => {
                setMode(tab);
                setInsert(null);
              }}
              className={cn(
                "rounded-lg px-3 py-2 text-xs font-semibold",
                mode === tab
                  ? "bg-primary text-on-primary"
                  : "text-ink-muted hover:text-ink",
              )}
            >
              {tab === "rich"
                ? "Rich text"
                : tab === "markdown"
                  ? "Markdown"
                  : "Preview"}
            </button>
          ))}
        </div>
      </div>
      <div
        className={cn(
          "min-w-0 overflow-hidden rounded-2xl border bg-surface-raised",
          error
            ? "border-danger"
            : "border-border-strong focus-within:border-primary",
        )}
      >
        {mode === "rich" && (
          <div
            role="toolbar"
            aria-label="Text formatting"
            className="flex flex-wrap items-center gap-1 border-b border-border bg-surface-sunken/70 p-2"
          >
            <BrandedSelect
              compact
              label="Text style"
              value={
                editor?.isActive("heading")
                  ? String(editor.getAttributes("heading").level)
                  : "paragraph"
              }
              options={[
                { value: "paragraph", label: "Normal text" },
                ...[1, 2, 3].map((n) => ({
                  value: String(n),
                  label: `Heading ${n}`,
                })),
              ]}
              onChange={(value) => {
                const level = Number(value) as 1 | 2 | 3;
                if (level) editor?.chain().focus().setHeading({ level }).run();
                else editor?.chain().focus().setParagraph().run();
              }}
            />
            {tools.map(({ label, icon: Icon, run, active, disabled }) => (
              <button
                key={label}
                type="button"
                title={label}
                aria-label={label}
                aria-pressed={active}
                disabled={disabled}
                onClick={run}
                className={cn(
                  "flex size-9 items-center justify-center rounded-lg disabled:opacity-30",
                  active
                    ? "bg-primary text-on-primary"
                    : "text-ink-muted hover:bg-surface hover:text-primary",
                )}
              >
                <Icon size={17} />
              </button>
            ))}
            {editor?.isActive("table") && (
              <div className="flex w-full flex-wrap gap-2 border-t border-border pt-2">
                {[
                  ["Add row", () => editor.chain().focus().addRowAfter().run()],
                  [
                    "Add column",
                    () => editor.chain().focus().addColumnAfter().run(),
                  ],
                  [
                    "Delete row",
                    () => editor.chain().focus().deleteRow().run(),
                  ],
                  [
                    "Delete column",
                    () => editor.chain().focus().deleteColumn().run(),
                  ],
                  [
                    "Delete table",
                    () => editor.chain().focus().deleteTable().run(),
                  ],
                ].map(([label, run]) => (
                  <button
                    key={String(label)}
                    type="button"
                    onClick={run as () => void}
                    className="rounded-lg border border-border px-2 py-1 text-xs text-ink"
                  >
                    {String(label)}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
        {mode === "rich" && insert === "link" && (
          <div className="flex flex-wrap items-end gap-2 border-b border-border p-4">
            <label className="flex min-w-0 flex-1 flex-col gap-1 text-xs text-ink">
              Link address
              <input
                aria-label="Link address"
                value={url}
                onChange={(e) => {
                  setUrl(e.target.value);
                  setLinkError("");
                }}
                placeholder="https://example.com"
                className="w-full rounded-lg border border-border bg-surface p-2 text-sm"
              />
            </label>
            <button
              type="button"
              className="rounded-lg bg-primary px-3 py-2 text-sm text-on-primary"
              onClick={() => {
                if (
                  !/^(https?:\/\/|mailto:|tel:|\/(?!\/)|#)/i.test(url.trim())
                ) {
                  setLinkError(
                    "Enter a web, email, phone or relative page address.",
                  );
                  return;
                }
                editor
                  ?.chain()
                  .focus()
                  .extendMarkRange("link")
                  .setLink({ href: url.trim() })
                  .run();
                setInsert(null);
              }}
            >
              Apply link
            </button>
            {linkError && (
              <p role="alert" className="w-full text-xs text-danger-ink">
                {linkError}
              </p>
            )}
          </div>
        )}
        {mode === "rich" && insert === "image" && (
          <div className="border-b border-border p-4">
            <label className="mt-3 block text-xs text-ink">
              Image description
              <input
                aria-label="Image description"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                className="ml-2 rounded border border-border bg-surface p-2"
              />
            </label>
            <ImagePicker
              value=""
              onChange={(src) => {
                if (src) {
                  editor
                    ?.chain()
                    .focus()
                    .setImage({ src, alt: url.trim() })
                    .run();
                  setInsert(null);
                }
              }}
            />

          </div>
        )}
        <div hidden={mode !== "rich"} className="overflow-x-auto">
          <EditorContent editor={editor} />
        </div>
        {mode === "markdown" && (
          <textarea
            aria-labelledby={id}
            aria-invalid={Boolean(error)}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            spellCheck={false}
            className="min-h-[26rem] w-full resize-y bg-transparent px-5 py-5 font-mono text-sm leading-7 text-ink outline-none"
          />
        )}
        {mode === "preview" && (
          <article
            aria-label="Body preview"
            className="terios-markdown min-h-[26rem] overflow-x-auto px-5 py-5"
          >
            {value.trim() ? (
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{value}</ReactMarkdown>
            ) : (
              <p className="text-ink-faint">Nothing to preview yet.</p>
            )}
          </article>
        )}
      </div>
      {error && (
        <p role="alert" className="text-sm text-danger-ink">
          {error}
        </p>
      )}
    </div>
  );
}
