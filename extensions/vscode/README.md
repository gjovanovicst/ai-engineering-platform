# VS Code companion (skeleton)

Commands: **AIP: List tasks / Start run / Show run / Decide approval / Show run evidence / Show run diff / Watch run**.
Configure `aip.apiUrl`, `aip.orgId`, `aip.token`, optional `aip.projectId`.

The token is stored in VS Code settings, so prefer a scoped API key with an
expiry over a session token. This is a first slice; evidence is a rendered markdown document and the diff opens **side-by-side** against the workspace file, with a picker for multi-file patches (falls back to the patch text on mismatch); watch run polls the event stream every 2s into an output channel (no SSE/terminal renderer).
