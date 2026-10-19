import type { ReactNode } from "react";

export function PageHeader({ title, intro, eyebrow, children }: { title: string; intro?: string; eyebrow?: string; children?: ReactNode }) {
  return (
    <div className="mx-auto max-w-7xl px-4 pb-8 pt-14 sm:px-6 sm:pt-20">
      {eyebrow && <p className="mb-3 font-mono text-xs uppercase tracking-[0.18em] text-accent-text">{eyebrow}</p>}
      <h1 className="max-w-4xl text-balance text-4xl font-semibold tracking-tight hyphens-auto sm:text-5xl">{title}</h1>
      {intro && <p className="mt-4 max-w-3xl text-pretty text-lg text-muted-foreground">{intro}</p>}
      {children}
    </div>
  );
}

export function Section({ id, title, intro, children, className }: { id?: string; title?: string; intro?: string; children: ReactNode; className?: string }) {
  return (
    <section id={id} aria-labelledby={id ? `${id}-h` : undefined} className={`mx-auto max-w-7xl px-4 py-12 sm:px-6 ${className ?? ""}`}>
      {title && (
        <div className="mb-8 max-w-3xl">
          <h2 id={id ? `${id}-h` : undefined} className="text-balance text-3xl font-semibold tracking-tight">{title}</h2>
          {intro && <p className="mt-3 text-pretty text-muted-foreground">{intro}</p>}
        </div>
      )}
      {children}
    </section>
  );
}
