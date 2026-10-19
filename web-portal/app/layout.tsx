import type { ReactNode } from "react";
import "./globals.css";

// The <html> element lives in app/[locale]/layout.tsx so it can carry lang, dir and the locale's fonts.
export default function RootLayout({ children }: { children: ReactNode }) {
  return children;
}
