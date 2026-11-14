const assert = require("node:assert");
const { parseFiles, applyPatchFor } = require("./patch");

const patch = [
  "diff --git a/src/a.ts b/src/a.ts",
  "--- a/src/a.ts",
  "+++ b/src/a.ts",
  "@@ -1,3 +1,4 @@",
  " one",
  " two",
  "+inserted",
  " three",
  "diff --git a/src/b.ts b/src/b.ts",
  "--- a/src/b.ts",
  "+++ b/src/b.ts",
  "@@ -1,1 +1,1 @@",
  "-old",
  "+new",
].join("\n");

const files = parseFiles(patch);
assert.deepEqual(files.map((f) => f.path), ["src/a.ts", "src/b.ts"]);
assert.equal(applyPatchFor("one\ntwo\nthree\n", patch, "src/a.ts").content, "one\ntwo\ninserted\nthree\n");
assert.equal(applyPatchFor("old\n", patch, "src/b.ts").content, "new\n");
console.log("patch test ok");
