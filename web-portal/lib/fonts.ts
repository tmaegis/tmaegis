import { GeistMono } from "geist/font/mono";
import { GeistSans } from "geist/font/sans";

// Geist (Latin, Vietnamese, Cyrillic, Serbian, Turkish) is the only font in the initial CSS. The CJK Noto fonts
// are loaded lazily by components/cjk-font.tsx and only on zh-Hans, zh-Hant, ja and ko.
export const fontClass = `${GeistSans.variable} ${GeistMono.variable}`;
