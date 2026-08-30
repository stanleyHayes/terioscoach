import type { ReactNode } from "react";

export default function PortalTemplate({ children }: { children: ReactNode }) {
  return <div className="terios-route-stage min-h-0 flex-1">{children}</div>;
}
