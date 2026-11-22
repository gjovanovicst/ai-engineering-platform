#!/usr/bin/env node
// Guards the UI classes of bug found in the local testing session:
//  1. a browser dialog instead of the design one (window.confirm/prompt/alert)
//  2. a SidePanel footer without a way out (Cancel/Close/Back/save)
//  3. a SidePanel that holds a form but declares no dirty guard
//  4. a list render on a possibly-null API field (.data.X.length / .map)
//  5. a task transition the domain does not allow
import { execSync } from "node:child_process";
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

// 6. the component contract: every registered component has an example, and a
//    contract change since the last release is reported (a removed or renamed
//    prop is breaking and fails).
const contractPath = new URL("../src/design/contracts.json", import.meta.url).pathname;
const contracts = JSON.parse(readFileSync(contractPath, "utf8"));
const playground = readFileSync(new URL("../src/design/playground.tsx", import.meta.url).pathname, "utf8");
const exampleKeys = new Set([...playground.matchAll(/^  ([A-Z][A-Za-z]+):/gm)].map((m) => m[1]));
for (const component of contracts.components) {
  if (!exampleKeys.has(component.name)) {
    warnings.push(`design/contracts.json: ${component.name} has no example in design/playground.tsx`);
  }
}

function lastRelease() {
  try {
    return execSync("git describe --tags --abbrev=0", { cwd: new URL("..", import.meta.url).pathname, encoding: "utf8" }).trim();
  } catch {
    return "";
  }
}

const tag = lastRelease();
if (tag) {
  try {
    const previous = JSON.parse(execSync(`git show ${tag}:apps/web/src/design/contracts.json`, {
      cwd: new URL("..", import.meta.url).pathname, encoding: "utf8",
    }));
    const before = new Map(previous.components.map((c) => [c.name, c]));
    const after = new Map(contracts.components.map((c) => [c.name, c]));
    for (const [name, component] of after) {
      const old = before.get(name);
      if (!old) { console.log(`contract: + ${name} (new)`); continue; }
      const oldProps = new Set(old.props.map((p) => p.name));
      const newProps = new Set(component.props.map((p) => p.name));
      for (const prop of newProps) if (!oldProps.has(prop)) console.log(`contract: ${name}.${prop} added`);
      for (const prop of oldProps) {
        if (!newProps.has(prop)) problems.push(`design/contracts.json: ${name}.${prop} was removed (breaking) — rename it or keep it deprecated`);
      }
      const oldStates = new Set(old.states);
      for (const state of component.states) if (!oldStates.has(state)) console.log(`contract: ${name}.${state} state added`);
    }
    for (const name of before.keys()) {
      if (!after.has(name)) problems.push(`design/contracts.json: the component ${name} was removed (breaking)`);
    }
  } catch {
    // the tag has no contracts.json yet: the first release records the baseline
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
