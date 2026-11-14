// @vitest-environment jsdom
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import axe from "axe-core";
import { describe, expect, it } from "vitest";

/**
 * Runs axe against the prerendered HTML of the public pages. Colour contrast is
 * disabled because jsdom has no layout/computed styles; it is verified
 * manually (docs/19-a11y.md).
 */
const pages = [
  { name: "login", file: ".next/server/app/login.html" },
  { name: "register", file: ".next/server/app/register.html" },
  { name: "not-found", file: ".next/server/app/_not-found.html" },
  { name: "design-system", file: ".next/server/app/design-system.html" },
  { name: "components", file: ".next/server/app/components.html" },
];

describe("accessibility of built pages", () => {
  for (const page of pages) {
    const path = resolve(process.cwd(), page.file);
    it.skipIf(!existsSync(path))(`${page.name} has no serious axe violations`, async () => {
      const html = readFileSync(path, "utf8").replace(/<script[\s\S]*?<\/script>/gi, "");
      document.open();
      document.write(html);
      document.close();

      const results = await axe.run(document, {
        rules: { "color-contrast": { enabled: false } },
        resultTypes: ["violations"],
      });

      const blocking = results.violations.filter(
        (violation) => violation.impact === "critical" || violation.impact === "serious",
      );
      expect(
        blocking.map((violation) => `${violation.id}: ${violation.help}`),
      ).toEqual([]);
    });
  }
});
