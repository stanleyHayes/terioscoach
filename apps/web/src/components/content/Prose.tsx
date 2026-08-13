import { cn } from "@/lib/cn";

/**
 * Long-form body renderer for CMS content (design-system §1 type scale).
 *
 * The CMS stores plain text with blank-line paragraph breaks, and this
 * renders exactly that: paragraphs and nothing else. It deliberately does
 * **not** render HTML. Article bodies are written in the dashboard, but a
 * compromised practitioner account — or a future import — would otherwise
 * be one field away from executing script on every visitor's browser. If
 * rich formatting is wanted later, the right move is a structured block
 * format with a whitelist, not `dangerouslySetInnerHTML` here.
 */
export interface ProseProps {
  body: string;
  className?: string;
}

export function Prose({ body, className }: ProseProps) {
  const paragraphs = body
    .split(/\n{2,}/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean);

  if (paragraphs.length === 0) {
    return null;
  }

  return (
    <div className={cn("flex max-w-[68ch] flex-col gap-5", className)}>
      {paragraphs.map((paragraph, index) => (
        <p
          key={index}
          className="text-base leading-[1.7] text-ink-muted [text-wrap:pretty] whitespace-pre-line"
        >
          {paragraph}
        </p>
      ))}
    </div>
  );
}
