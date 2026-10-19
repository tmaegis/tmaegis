import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva("inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap", {
  variants: {
    variant: {
      default: "border-border bg-muted text-foreground",
      accent: "border-accent/50 bg-accent/10 text-accent-text",
      primary: "border-primary/40 bg-primary/10 text-primary",
      critical: "border-destructive bg-destructive/10 text-destructive",
      high: "border-destructive/60 text-destructive",
      medium: "border-warning/70 text-warning",
      low: "border-success/60 text-success",
    },
  },
  defaultVariants: { variant: "default" },
});

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}
