import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  "inline-flex max-w-full items-center justify-center gap-2 rounded-lg text-center text-sm font-semibold leading-tight transition-[background,box-shadow,border-color,opacity] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground shadow-[0_0_0_1px_rgb(0_242_254/0.25),0_8px_28px_-10px_var(--primary)] hover:opacity-90",
        outline: "border border-border-strong bg-card text-foreground hover:border-primary hover:bg-muted",
        ghost: "text-foreground hover:bg-muted",
      },
      // min-height, not height: a long translation wraps inside the button instead of overflowing the page.
      size: { default: "min-h-10 px-4 py-2", sm: "min-h-9 px-3 py-1.5", lg: "min-h-12 px-6 py-2.5 text-base" },
    },
    defaultVariants: { variant: "default", size: "default" },
  },
);

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export function Button({ className, variant, size, asChild = false, ...props }: ButtonProps) {
  const Comp = asChild ? Slot : "button";
  return <Comp className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
