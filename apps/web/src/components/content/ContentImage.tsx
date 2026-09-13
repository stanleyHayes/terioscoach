import Image, { type ImageProps } from "next/image";

/** An intentionally cleared CMS image leaves its decorative frame empty. */
export function ContentImage(props: ImageProps) {
  if (!props.src) return null;
  return <Image {...props} alt={props.alt} />;
}
