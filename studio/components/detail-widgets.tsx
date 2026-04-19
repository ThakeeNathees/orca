"use client";

// Shared dashboard/tab widgets used across entity detail pages.

export function StatCard({
  title,
  value,
}: {
  title: string;
  value: string;
}) {
  return (
    <div className="flex-1 rounded-lg border border-border bg-card p-4">
      <div className="text-caption uppercase tracking-wider text-muted-foreground">
        {title}
      </div>
      <div className="mt-2 text-sm text-foreground">{value}</div>
    </div>
  );
}

export function CostRow() {
  return (
    <div className="flex gap-6 rounded-lg border border-border bg-card p-4">
      <CostCell label="Input Tokens" value="0" />
      <CostCell label="Output Tokens" value="0" />
      <CostCell label="Cached Tokens" value="0" />
      <CostCell label="Total Cost" value="$0.00" />
    </div>
  );
}

function CostCell({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex-1">
      <div className="text-caption uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="mt-1 text-lg font-medium text-foreground">{value}</div>
    </div>
  );
}

export function DashboardSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h3 className="mb-2 text-sm font-medium text-foreground">{title}</h3>
      {children}
    </section>
  );
}
