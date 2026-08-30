import { cn } from "@/lib/cn";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

/**
 * Long-form body renderer for CMS content (design-system §1 type scale).
 *
 * Markdown is rendered into React elements without raw HTML support. That
 * keeps links, lists, headings and emphasis useful while an injected HTML or
 * script fragment remains inert text instead of executable page content.
 */
export interface ProseProps {
  body: string;
  className?: string;
}

export function Prose({ body, className }: ProseProps) {
  if (!body.trim()) return null;

  return (
    <div className={cn("terios-markdown max-w-[68ch]", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{body}</ReactMarkdown>
    </div>
  );
}
