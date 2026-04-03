import type { NextConfig } from "next";

// Subpath on GitHub Pages: VitePress `base` is `/orca/` (docs/.vitepress/config.ts).
const basePath = "/orca/studio";

const nextConfig: NextConfig = {
  output: "export",
  basePath,
};

export default nextConfig;
