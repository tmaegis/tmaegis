"use client";

import { useEffect } from "react";
import { Noto_Sans_TC } from "next/font/google";

// Variable Noto font, display swap, never preloaded. This chunk (and its @font-face CSS) is only fetched on this locale.
const noto = Noto_Sans_TC({ display: "swap", preload: false, fallback: ["system-ui", "sans-serif"], variable: "--font-noto-tc" });

export default function Font() {
  useEffect(() => {
    document.documentElement.classList.add(noto.variable);
  }, []);
  return null;
}
