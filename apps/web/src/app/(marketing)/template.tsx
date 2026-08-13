import type { ReactNode } from "react";

export default function MarketingTemplate({ children }: { children: ReactNode }) {
  return <div className="terios-route-stage flex min-h-0 flex-1 flex-col">{children}</div>;
}
