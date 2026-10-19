"use client";

import { useEffect } from "react";
import { Noto_Sans_KR } from "next/font/google";

// Variable Noto font, display swap, never preloaded. This chunk (and its @font-face CSS) is only fetched on this locale.
const noto = Noto_Sans_KR({ display: "swap", preload: false, fallback: ["system-ui", "sans-serif"], variable: "--font-noto-kr" });

export default function Font() {
  useEffect(() => {
    document.documentElement.classList.add(noto.variable);
  }, []);
  return null;
}
