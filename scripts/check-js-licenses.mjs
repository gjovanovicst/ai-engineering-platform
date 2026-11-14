#!/usr/bin/env node
// Fail when a production npm dependency uses a license outside the allowlist.
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const allowlist = new Set(JSON.parse(readFileSync(path.join(root, "deploy/licenses/allowlist.json"), "utf8")).allowed);
let raw;
try {
  raw = execFileSync("pnpm", ["licenses", "list", "--prod", "--json"], { cwd: path.join(root, "apps/web"), encoding: "utf8" });
} catch (error) {
  console.error("js-licenses: pnpm licenses failed:", error.message);
  process.exit(1);
}
const byLicense = JSON.parse(raw);
const violations = Object.keys(byLicense).filter((license) => !allowlist.has(license));
if (violations.length > 0) {
  for (const license of violations) {
    const packages = byLicense[license].map((pkg) => `${pkg.name}@${pkg.versions.join(",")}`).join(", ");
    console.error(`js-licenses: disallowed ${license}: ${packages}`);
  }
  process.exit(1);
}
console.log(`js-licenses: ${Object.keys(byLicense).length} license types allowed`);
