"use client";

import dynamic from "next/dynamic";

// One lazy chunk per CJK script. Latin, Cyrillic, Vietnamese, Serbian and Turkish locales use Geist only and load none of this.
const fonts = {
  hans: dynamic(() => import("./fonts/hans"), { ssr: false }),
  hant: dynamic(() => import("./fonts/hant"), { ssr: false }),
  ja: dynamic(() => import("./fonts/ja"), { ssr: false }),
  ko: dynamic(() => import("./fonts/ko"), { ssr: false }),
} as const;

export function CjkFont({ script }: { script: string }) {
  const Font = fonts[script as keyof typeof fonts];
  return Font ? <Font /> : null;
}
