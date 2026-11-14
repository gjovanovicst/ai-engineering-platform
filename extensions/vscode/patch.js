// Minimal unified-diff parser/applier used by the VS Code companion.
function parseFiles(text) {
  const lines = String(text).split("\n");
  const files = [];
  let file;
  let hunk;
  for (const line of lines) {
    if (line.startsWith("diff --git ")) {
      file = { path: null, hunks: [] };
      files.push(file);
      hunk = null;
      continue;
    }
    if (line.startsWith("+++ ")) {
      if (!file) {
        file = { path: null, hunks: [] };
        files.push(file);
      }
      file.path = line.slice(4).replace(/^b\//, "").trim();
      hunk = null;
      continue;
    }
    const match = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/.exec(line);
    if (match && file) {
      hunk = { oldStart: Number(match[1]), lines: [] };
      file.hunks.push(hunk);
      continue;
    }
    if (hunk && (line.startsWith("+") || line.startsWith("-") || line.startsWith(" "))) {
      hunk.lines.push(line);
    }
  }
  return files.filter((entry) => entry.path && entry.hunks.length > 0);
}

function applyFile(base, file) {
  const source = String(base).split("\n");
  const out = [];
  let pos = 0;
  for (const hunk of file.hunks) {
    const start = hunk.oldStart - 1;
    while (pos < start) out.push(source[pos++]);
    for (const line of hunk.lines) {
      const kind = line[0];
      const body = line.slice(1);
      if (kind === " ") {
        if (source[pos] !== body) throw new Error("patch context does not match workspace file");
        out.push(body);
        pos++;
      } else if (kind === "-") {
        if (source[pos] !== body) throw new Error("patch removal does not match workspace file");
        pos++;
      } else if (kind === "+") {
        out.push(body);
      }
    }
  }
  while (pos < source.length) out.push(source[pos++]);
  return { file: file.path, content: out.join("\n") };
}

function applyPatchFor(base, patchText, wanted) {
  const files = parseFiles(patchText);
  const file = wanted ? files.find((entry) => entry.path === wanted) : files[0];
  if (!file) throw new Error("patch has no matching file");
  return applyFile(base, file);
}

function applyPatch(base, patchText) {
  return applyPatchFor(base, patchText, null);
}

module.exports = { parseFiles, applyPatchFor, applyPatch };
