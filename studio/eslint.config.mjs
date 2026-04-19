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
      // Allow leading-underscore names as the "intentionally unused"
      // convention (destructure-and-discard, unused args for interface
      // compliance, etc.). Matches @typescript-eslint's recommendation.
      "@typescript-eslint/no-unused-vars": [
        "warn",
        {
          args: "after-used",
          argsIgnorePattern: "^_",
          varsIgnorePattern: "^_",
          caughtErrorsIgnorePattern: "^_",
          destructuredArrayIgnorePattern: "^_",
          ignoreRestSiblings: true,
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
