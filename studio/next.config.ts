import type { NextConfig } from "next";

// GitHub Pages hosts Studio under `/orca/studio` (see docs `.vitepress` base `/orca/`).
// `package.json` sets `ORCA_STUDIO_BASE_PATH` only for `pnpm run build`, so `next dev`
// serves at `http://localhost:3000/` with no `basePath`. Relying on `NODE_ENV` here
// is unreliable because Next may load this config before it flips to `production`.
// `output: "export"` forbids middleware/proxy.
const basePath = process.env.ORCA_STUDIO_BASE_PATH ?? "";

const nextConfig: NextConfig = {
  output: "export",
  ...(basePath ? { basePath } : {}),
};

export default nextConfig;
