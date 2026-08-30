"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

/** Immediate visual acknowledgement while a dynamic dashboard route streams. */
export function RouteFeedback() {
  const pathname = usePathname();
  const router = useRouter();
  const [pendingPath, setPendingPath] = useState<string | null>(null);
  useEffect(() => {
    function localPath(target: EventTarget | null) {
      const anchor = target instanceof Element ? target.closest("a[href]") : null;
      if (!(anchor instanceof HTMLAnchorElement) || anchor.target === "_blank" || anchor.hasAttribute("download")) return null;
      const url = new URL(anchor.href, window.location.href);
      return url.origin === window.location.origin ? url.pathname : null;
    }
    function warm(event: Event) {
      const href = localPath(event.target);
      if (href && href !== pathname) router.prefetch(href);
    }
    function begin(event: MouseEvent) {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const href = localPath(event.target);
      if (href && href !== pathname) setPendingPath(href);
    }
    document.addEventListener("pointerover", warm, true);
    document.addEventListener("focusin", warm, true);
    document.addEventListener("click", begin, true);
    return () => {
      document.removeEventListener("pointerover", warm, true);
      document.removeEventListener("focusin", warm, true);
      document.removeEventListener("click", begin, true);
    };
  }, [pathname, router]);

  return <div className="terios-route-progress" data-active={(pendingPath !== null && pendingPath !== pathname) || undefined} aria-hidden="true"><span /></div>;
}
