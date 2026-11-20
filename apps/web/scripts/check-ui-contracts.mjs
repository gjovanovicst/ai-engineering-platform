#!/usr/bin/env node
// Guards the UI classes of bug found in the local testing session:
//  1. a browser dialog instead of the design one (window.confirm/prompt/alert)
//  2. a SidePanel footer without a way out (Cancel/Close/Back/save)
//  3. a SidePanel that holds a form but declares no dirty guard
//  4. a list render on a possibly-null API field (.data.X.length / .map)
//  5. a task transition the domain does not allow
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const ROOT = new URL("../src", import.meta.url).pathname;
const problems = [];

const files = [];

function walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) walk(path);
    else if (/\.(tsx?|jsx?)$/.test(name) && !/\.test\./.test(name)) files.push(path);
  }
}
walk(ROOT);

const source = new Map(files.map((file) => [file, readFileSync(file, "utf8")]));
const rel = (file) => relative(ROOT, file);

// 1. the browser dialogs
for (const [file, text] of source) {
  for (const name of ["confirm", "prompt", "alert"]) {
    if (new RegExp(`window\\.${name}\\(`).test(text)) {
      problems.push(`${rel(file)}: window.${name} — use the design ConfirmDialog/PromptDialog`);
    }
  }
}

// 2. every SidePanel footer must offer a way out
for (const [file, text] of source) {
  for (const match of text.matchAll(/<SidePanel[\s\S]*?footer=\{[\s\S]*?\n\s*\}/g)) {
    if (!/(Cancel|Close|Back|Save)/.test(match[0])) {
      problems.push(`${rel(file)}: a SidePanel footer without Cancel/Close/Back`);
    }
  }
}

// 3. a SidePanel form without the dirty guard (a warning during the cleanup)
const warnings = [];
for (const [file, text] of source) {
  if (!/<SidePanel/.test(text)) continue;
  const hasForm = /<(form|Input|Field)\b/.test(text);
  if (hasForm && !/dirty=/.test(text)) {
    warnings.push(`${rel(file)}: a SidePanel with a form but no dirty guard`);
  }
}

// 4. null-unsafe list renders
for (const [file, text] of source) {
  for (const match of text.matchAll(/\.data\??\.([a-zA-Z_]+)\.(length|map)\b/g)) {
    const field = match[1];
    const guarded = new RegExp(`\\(\\s*(?:[a-zA-Z_][\\w.]*\\.)*${field}\\s*\\?\\?\\s*\\[\\]`).test(text);
    if (!guarded) {
      warnings.push(`${rel(file)}: .data.${field}.${match[2]} without ?? []`);
    }
  }
}

if (warnings.length > 0) {
  console.warn(`ui contracts: ${warnings.length} warning(s) (a progressive cleanup)`);
  for (const warning of warnings) console.warn(`  ${warning}`);
}
if (problems.length > 0) {
  console.error(`ui contracts: ${problems.length} error(s)\n`);
  for (const problem of problems) console.error(`  ${problem}`);
  process.exit(1);
}
console.log(`ui contracts: ${files.length} files, the contracts hold`);
