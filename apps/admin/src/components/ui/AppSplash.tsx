export function AppSplash({ label = "Practice workspace" }: { label?: string }) {
  return <main className="terios-splash" role="status" aria-live="polite" aria-label={`Opening ${label}`}>
    <div className="terios-splash-orbit" aria-hidden="true"><span className="terios-splash-leaf terios-splash-leaf-left"/><span className="terios-splash-leaf terios-splash-leaf-right"/><span className="terios-splash-core"/></div>
    <div className="terios-splash-copy"><p>Terios</p><span>{label}</span></div>
    <div className="terios-splash-progress" aria-hidden="true"><span/></div>
    <span className="sr-only">Preparing {label}…</span>
  </main>;
}
