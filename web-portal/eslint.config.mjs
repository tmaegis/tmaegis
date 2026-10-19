import next from "eslint-config-next";

const config = [
  ...next,
  { ignores: [".next/**", ".next-*/**", "node_modules/**", "public/**", "generated/**", "next-env.d.ts", "scripts/**", "**/*.d.mts"] },
  { rules: { "react/no-unescaped-entities": "off" } },
];
export default config;
