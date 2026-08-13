import type { ReactNode } from "react";

export default function AuthTemplate({ children }: { children: ReactNode }) {
  return <div className="terios-route-stage min-h-full">{children}</div>;
}
