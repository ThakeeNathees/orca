import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    rules: {
      // Ban div[role="button"] — use real <button> elements instead.
      "no-restricted-syntax": [
        "warn",
        {
          selector: 'JSXAttribute[name.name="role"][value.value="button"]',
          message:
            'Avoid role="button" on non-button elements. Use a <button> instead for proper accessibility.',
        },
      ],
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
]);

export default eslintConfig;
