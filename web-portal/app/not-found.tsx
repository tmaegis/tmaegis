import Link from "next/link";
import en from "@/messages/en-US.json";
import { GeistSans } from "geist/font/sans";

// Fallback for URLs outside any locale (the proxy normally redirects those to /<locale>/...).
export default function RootNotFound() {
  return (
    <html lang="en-US" className={GeistSans.variable}>
      <body style={{ fontFamily: "var(--font-geist-sans), system-ui, sans-serif", background: "#0b0f19", color: "#e5e7eb", margin: 0 }}>
        <main style={{ minHeight: "100dvh", display: "grid", placeItems: "center", padding: "2rem", textAlign: "center" }}>
          <div>
            <h1 style={{ fontSize: "2rem", margin: "0 0 .5rem" }}>404</h1>
            <p style={{ color: "#9aa8bd" }}>{en.notFound.title}</p>
            <Link href="/en" style={{ color: "#00f2fe" }}>TMAegis</Link>
          </div>
        </main>
      </body>
    </html>
  );
}
