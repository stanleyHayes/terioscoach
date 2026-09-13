"use client";
import { createContext, useContext, type ReactNode } from "react";
import { siteCopy, type SiteValues } from "../../../../shared/site-content";
const SiteCopyContext = createContext<Record<string, SiteValues>>({});
export function SiteCopyProvider({
  values,
  children,
}: {
  values: Record<string, SiteValues>;
  children: ReactNode;
}) {
  return (
    <SiteCopyContext.Provider value={values}>
      {children}
    </SiteCopyContext.Provider>
  );
}
export function useSiteCopy(scope: string) {
  return siteCopy(scope, useContext(SiteCopyContext)[scope]);
}
