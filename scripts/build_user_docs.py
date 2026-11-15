#!/usr/bin/env python3
"""Build the static documentation portal under site/docs.

Source of truth: docs/*.md, docs/adr/*.md and api/openapi.yaml.
No third-party markdown package: the renderer covers the constructs used here.
"""
from __future__ import annotations

import html
import json
import re
import shutil
from pathlib import Path
from urllib.parse import urlsplit

import yaml

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs" / "user"
OUT = ROOT / "site" / "docs"
PAGES = OUT / "pages"

SITE_NAME = "AI Engineering Platform"
REPO_BLOB = "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/"
REPO_TREE = "https://github.com/gjovanovicst/ai-engineering-platform/tree/main/"
GROUPS = [
    ("start", "Getting started"),
    ("work", "Build & ship"),
    ("quality", "Quality & insights"),
    ("admin", "Administration"),
    ("api", "API & integrations"),
]
GROUP_ORDER = {key: i for i, (key, _) in enumerate(GROUPS)}
GROUP_FOR = {
    "overview": "start", "quickstart": "start", "concepts": "start",
    "projects-blueprints": "work", "tasks-and-board": "work", "runs-and-approvals": "work",
    "pull-requests": "work", "git-providers": "work", "quality": "quality", "analytics": "quality",
    "users-access": "admin", "security-audit": "admin", "models-byok": "admin",
    "backups-recovery": "admin", "troubleshooting": "admin",
    "api-basics": "api",
}


def esc(value: object) -> str:
    return html.escape(str(value or ""), quote=True)


def slugify(text: str) -> str:
    s = re.sub(r"[`*_~]", "", text).lower()
    s = re.sub(r"[^a-z0-9\u0400-\u04ff]+", "-", s).strip("-")
    return s or "section"


def group_for(slug: str) -> str:
    return GROUP_FOR.get(slug, "admin")


def natural_key(slug: str):
    return re.sub(r"\d+", lambda m: f"{int(m.group()):08d}", slug)


class Markdown:
    def __init__(self, pages: dict[str, str]):
        self.pages = pages
        self.headings: list[tuple[int, str, str]] = []
        self.used_ids: set[str] = set()

    def anchor(self, text: str) -> str:
        base = slugify(text)
        candidate = base
        i = 2
        while candidate in self.used_ids:
            candidate = f"{base}-{i}"
            i += 1
        self.used_ids.add(candidate)
        return candidate

    def link(self, href: str) -> str:
        if not href or href.startswith(("#", "http://", "https://", "mailto:")):
            return href
        parts = urlsplit(href)
        path, frag = parts.path, parts.fragment
        if path.endswith(".md"):
            name = Path(path).name
            stem = name[:-3]
            if path.startswith("../api/events"):
                stem = "api-events"
            slug = f"adr-{stem}" if Path(path).parent.name == "adr" else stem
            if slug in self.pages:
                return f"{slug}.html" + (f"#{frag}" if frag else "")
        if path.endswith(".yaml") or path.endswith(".yml"):
            return "../openapi.yaml" + (f"#{frag}" if frag else "")
        if path.startswith("../AGENTS.md"):
            return REPO_BLOB + "AGENTS.md" + (f"#{frag}" if frag else "")
        if path.startswith("../design/"):
            return REPO_TREE + path[3:].rstrip("/")
        if path.startswith("../"):
            return href
        return href

    def inline(self, text: str) -> str:
        codes: list[str] = []

        def keep_code(m):
            codes.append(m.group(1))
            return f"\x00{len(codes)-1}\x00"

        text = re.sub(r"`([^`\n]+)`", keep_code, text)
        text = esc(text)
        text = re.sub(r"!\[([^\]]*)\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)",
                      lambda m: f'<img src="{esc(self.link(m.group(2)))}" alt="{esc(m.group(1))}" />', text)
        text = re.sub(r"\[([^\]]+)\]\(([^)\s]+)(?:\s+\"[^\"]*\")?\)",
                      lambda m: f'<a href="{esc(self.link(m.group(2)))}">{m.group(1)}</a>', text)
        text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
        text = re.sub(r"(?<!\*)\*([^*\n]+)\*(?!\*)", r"<em>\1</em>", text)
        text = re.sub(r"~~([^~]+)~~", r"<del>\1</del>", text)
        for i, code in enumerate(codes):
            text = text.replace(f"\x00{i}\x00", f"<code>{esc(code)}</code>")
        return text

    def render(self, source: str) -> tuple[str, list[tuple[int, str, str]]]:
        lines = source.replace("\r\n", "\n").split("\n")
        out: list[str] = []
        i = 0
        while i < len(lines):
            line = lines[i]
            if not line.strip():
                i += 1
                continue

            if line.startswith("```"):
                lang = line[3:].strip()
                i += 1
                block: list[str] = []
                while i < len(lines) and not lines[i].startswith("```"):
                    block.append(lines[i])
                    i += 1
                i += 1
                cls = f' class="language-{esc(lang)}"' if lang else ""
                out.append(f"<pre><code{cls}>{esc(chr(10).join(block))}</code></pre>")
                continue

            m = re.match(r"^(#{1,6})\s+(.*?)\s*#*\s*$", line)
            if m:
                level = len(m.group(1))
                text = m.group(2)
                aid = self.anchor(text)
                self.headings.append((level, text, aid))
                out.append(f'<h{level} id="{aid}">{self.inline(text)}</h{level}>')
                i += 1
                continue

            if re.match(r"^\s*([-*_])(?:\s*\1){2,}\s*$", line):
                out.append("<hr />")
                i += 1
                continue

            if line.lstrip().startswith("|") and i + 1 < len(lines) and re.match(
                r"^\s*\|?[\s:|-]+\|[\s:|-]*$", lines[i + 1]
            ):
                header = [c.strip() for c in line.strip().strip("|").split("|")]
                aligns = []
                for c in lines[i + 1].strip().strip("|").split("|"):
                    c = c.strip()
                    aligns.append("center" if c.startswith(":") and c.endswith(":") else "right" if c.endswith(":") else "left")
                i += 2
                rows = []
                while i < len(lines) and lines[i].lstrip().startswith("|"):
                    rows.append([c.strip() for c in lines[i].strip().strip("|").split("|")])
                    i += 1
                head = "".join(f'<th style="text-align:{a}">{self.inline(c)}</th>' for c, a in zip(header, aligns))
                body = "".join("<tr>" + "".join(f'<td style="text-align:{a}">{self.inline(c)}</td>' for c, a in zip(row, aligns)) + "</tr>" for row in rows)
                out.append(f'<div class="table-wrap"><table><thead><tr>{head}</tr></thead><tbody>{body}</tbody></table></div>')
                continue

            if line.lstrip().startswith(">"):
                quote: list[str] = []
                while i < len(lines) and lines[i].lstrip().startswith(">"):
                    quote.append(re.sub(r"^\s*>\s?", "", lines[i]))
                    i += 1
                out.append(f"<blockquote>{self.paragraph(' '.join(quote).strip())}</blockquote>")
                continue

            if re.match(r"^\s*([-*+]|\d+\.)\s+", line):
                block, i = self.list_block(lines, i)
                out.append(block)
                continue

            para: list[str] = []
            while i < len(lines) and lines[i].strip() and not re.match(r"^(#{1,6})\s", lines[i]) \
                    and not lines[i].startswith("```") and not re.match(r"^\s*([-*+]|\d+\.)\s+", lines[i]) \
                    and not lines[i].lstrip().startswith("|") and not lines[i].lstrip().startswith(">"):
                para.append(lines[i].strip())
                i += 1
            out.append(self.paragraph(" ".join(para)))
        return "\n".join(out), self.headings

    def paragraph(self, text: str) -> str:
        return f"<p>{self.inline(text)}</p>" if text else ""

    def list_block(self, lines: list[str], start: int) -> tuple[str, int]:
        items: list[tuple[int, str, bool]] = []
        i = start
        while i < len(lines):
            m = re.match(r"^(\s*)([-*+]|\d+\.)\s+(.*)$", lines[i])
            if not m:
                if lines[i].strip() and items and lines[i].startswith(" "):
                    indent, text, ordered = items[-1]
                    items[-1] = (indent, text + " " + lines[i].strip(), ordered)
                    i += 1
                    continue
                break
            items.append((len(m.group(1)), m.group(3), m.group(2)[0].isdigit()))
            i += 1

        def render_items(index: int, indent: int) -> tuple[str, int]:
            ordered = items[index][2]
            tag = "ol" if ordered else "ul"
            parts = [f"<{tag}>"]
            while index < len(items) and items[index][0] == indent:
                _, text, _ = items[index]
                index += 1
                nested = ""
                if index < len(items) and items[index][0] > indent:
                    nested, index = render_items(index, items[index][0])
                parts.append(f"<li>{self.inline(text)}{nested}</li>")
            parts.append(f"</{tag}>")
            return "".join(parts), index

        rendered, _ = render_items(0, items[0][0])
        return rendered, i


def load_docs() -> list[dict]:
    records = []
    for path in sorted(DOCS.glob("*.md"), key=lambda p: natural_key(p.stem)):
        slug = re.sub(r"^\d+-", "", path.stem)
        text = path.read_text(encoding="utf-8")
        m = re.search(r"^#\s+(.+)$", text, re.M)
        title = m.group(1).strip() if m else slug.replace("-", " ").title()
        title = re.sub(r"[`*_]", "", title)
        first = next((ln.strip().lstrip("> ").strip() for ln in text.splitlines()
                      if ln.strip() and not ln.startswith("#") and not ln.startswith("```")), "")
        description = re.sub(r"[`*_\[\]()]", "", first)[:180]
        records.append({
            "slug": slug,
            "file": path.name,
            "source": f"docs/user/{path.name}",
            "title": title,
            "description": description,
            "group": group_for(slug),
            "order": natural_key(path.stem),
            "text": text,
        })
    records.sort(key=lambda r: (GROUP_ORDER.get(r["group"], 99), r["order"]))
    return records


def nav_html(records: list[dict], current: str, prefix: str) -> str:
    parts = []
    for key, label in GROUPS:
        items = [r for r in records if r["group"] == key]
        if not items:
            continue
        open_attr = " open" if any(r["slug"] == current for r in items) else ""
        links = "".join(
            f'<a class="{"active" if r["slug"] == current else ""}" href="{prefix}pages/{r["slug"]}.html">{esc(r["title"])}</a>'
            for r in items
        )
        parts.append(f'<details class="nav-group"{open_attr}><summary>{label} <span>{len(items)}</span></summary><div class="nav-items">{links}</div></details>')
    api_active = " active" if current == "api" else ""
    parts.append(f'<a class="nav-api{api_active}" href="{prefix}api.html">API reference <span>REST</span></a>')
    return "".join(parts)


def page_shell(title: str, body: str, records: list[dict], current: str, prefix: str,
               description: str = "", toc: str = "", body_class: str = "") -> str:
    return f'''<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>{esc(title)} · {SITE_NAME}</title>
<meta name="description" content="{esc(description)}" />
<link rel="icon" href="{prefix}../assets/favicon.svg" type="image/svg+xml" />
<link rel="stylesheet" href="{prefix}docs.css" />
<script>try{{var t=localStorage.getItem("aip-theme");var d=t?t==="dark":!window.matchMedia("(prefers-color-scheme: light)").matches;if(d)document.documentElement.dataset.theme="dark";else document.documentElement.dataset.theme="light";}}catch(e){{}}</script>
</head>
<body class="{body_class}">
<a class="skip" href="#doc-content">Skip to content</a>
<header class="topbar">
  <button class="menu-btn" type="button" aria-label="Open documentation menu">☰</button>
  <a class="doc-brand" href="{prefix}index.html">
    <svg viewBox="0 0 32 32" aria-hidden="true"><rect width="32" height="32" rx="8" fill="#080b12"/><path d="M7 24 16 5l9 19h-4.2l-1.6-3.7h-6.4L11.2 24H7Zm6.9-7h4.2L16 11.6 13.9 17Z" fill="#7c9cff"/><circle cx="16" cy="27" r="1.8" fill="#7c9cff"/></svg>
    <span><strong>User guide</strong><small>AI Engineering Platform</small></span>
  </a>
  <div class="top-search">
    <input id="doc-search" type="search" placeholder="Search…" autocomplete="off" aria-label="Search the user guide" />
    <div id="doc-results" class="search-results" hidden></div>
  </div>
  <div class="top-actions">
    <button class="theme-btn" type="button" aria-label="Toggle theme">◐</button>
    <a class="back-site" href="{prefix}../index.html"><span aria-hidden="true">←</span> <span class="back-long">Back to site</span><span class="back-short">Site</span></a>
  </div>
</header>
<div class="docs-layout">
  <aside class="docs-nav" aria-label="User guide">
    <nav>{nav_html(records, current, prefix)}</nav>
    <a class="nav-back" href="{prefix}../index.html"><span aria-hidden="true">←</span> Back to product site</a>
    <p class="nav-note">AI Engineering Platform · User guide</p>
  </aside>
  <div class="docs-main">
    <main id="doc-content" class="doc-content">
      {body}
    </main>
    {toc}
  </div>
</div>
<div class="nav-backdrop" hidden></div>
<script src="{prefix}docs.js" defer></script>
</body>
</html>'''


def doc_page(record: dict, records: list[dict]) -> str:
    md = Markdown({r["slug"]: r["file"] for r in records})
    body, headings = md.render(record["text"])
    toc_links = "".join(f'<a href="#{aid}" class="toc-l{lvl}">{esc(text)}</a>' for lvl, text, aid in headings if 1 < lvl <= 3)
    toc = f'<aside class="page-toc"><p>On this page</p>{toc_links}</aside>' if toc_links else ""
    breadcrumb = f'<p class="breadcrumb"><a href="../index.html">User guide</a> / {esc(dict(GROUPS).get(record["group"], "Reference"))}</p>'
    body_html = f'''{breadcrumb}
<article class="prose">
{body}
</article>
'''
    return page_shell(record["title"], body_html, records, record["slug"], "../",
                      record["description"], toc, "doc-page")


def index_page(records: list[dict]) -> str:
    spec = yaml.safe_load((ROOT / "api" / "openapi.yaml").read_text(encoding="utf-8"))
    path_count = len(spec.get("paths", {}))
    schema_count = len(spec.get("components", {}).get("schemas", {}))
    operation_count = sum(1 for item in spec.get("paths", {}).values()
                          for m in ("get", "post", "put", "patch", "delete") if item.get(m))
    cards = []
    for key, label in GROUPS:
        items = [r for r in records if r["group"] == key]
        if not items:
            continue
        links = "".join(
            f'<a class="doc-row" href="pages/{r["slug"]}.html"><span>{esc(r["title"])}</span><small>{esc(r["description"])}</small></a>'
            for r in items
        )
        cards.append(f'<section class="doc-group"><h2>{label} <span>{len(items)}</span></h2><div class="doc-rows">{links}</div></section>')
    intro = f'''<div class="docs-hero">
  <p class="eyebrow">Product guide</p>
  <h1>Do the work. Keep the controls.</h1>
  <p class="lede">Everything your team needs to set up a project, run an agent, approve a change and operate the workspace. The API reference is generated from the OpenAPI contract.</p>
  <div class="hero-links"><a class="primary" href="pages/quickstart.html">Start the quickstart</a><a href="pages/concepts.html">Key concepts</a><a href="api.html">API reference</a></div>
</div>
<div class="docs-stats"><div><strong>{len(records)}</strong><span>user guides</span></div><div><strong>{operation_count}</strong><span>API operations</span></div><div><strong>{schema_count}</strong><span>schemas</span></div><a href="../index.html">← Back to the product</a></div>
<div class="group-grid">{''.join(cards)}</div>'''
    return page_shell("User guide", intro, records, "index", "", "User guide for the AI Engineering Platform: setup, projects, tasks, runs, approvals, security and API.", "")


def schema_name(schema) -> str:
    if not isinstance(schema, dict):
        return ""
    ref = schema.get("$ref")
    if ref:
        return ref.rsplit("/", 1)[-1]
    if "oneOf" in schema:
        return "oneOf " + ", ".join(schema_name(s) for s in schema["oneOf"])
    if "anyOf" in schema:
        return "anyOf " + ", ".join(schema_name(s) for s in schema["anyOf"])
    t = schema.get("type", "object")
    if t == "array":
        return f"array&lt;{schema_name(schema.get('items', {}))}&gt;"
    return schema.get("format", t) if t == "string" and schema.get("format") else t


def schema_link(schema) -> str:
    name = schema_name(schema)
    if isinstance(schema, dict) and schema.get("$ref"):
        return f'<a href="#schema-{slugify(name)}"><code>{esc(name)}</code></a>'
    return f'<code>{name}</code>'


def render_api() -> str:
    spec = yaml.safe_load((ROOT / "api" / "openapi.yaml").read_text(encoding="utf-8"))
    md = Markdown({})
    tags = {t["name"]: t.get("description", "") for t in spec.get("tags", [])}
    ops_by_tag: dict[str, list] = {}
    for path, item in spec.get("paths", {}).items():
        for method in ("get", "post", "put", "patch", "delete"):
            op = item.get(method)
            if not op:
                continue
            tag = (op.get("tags") or ["other"])[0]
            ops_by_tag.setdefault(tag, []).append((path, method, op))

    toc = ['<a href="#authentication">Authentication</a>', '<a href="#operations">Operations</a>']
    toc += [f'<a href="#tag-{slugify(t)}">{esc(t)}</a>' for t in ops_by_tag]
    toc.append('<a href="#schemas">Schemas</a>')
    toc_html = f'<aside class="page-toc"><p>On this page</p>{"".join(toc)}</aside>'

    info = spec.get("info", {})
    schemes = spec.get("components", {}).get("securitySchemes", {})
    servers = spec.get("servers", [])
    operation_count = sum(len(v) for v in ops_by_tag.values())
    out = [f'''<p class="breadcrumb"><a href="index.html">User guide</a> / API reference</p>
<div class="api-hero">
  <p class="eyebrow">Developer reference · v{esc(info.get("version", "1"))}</p>
  <h1>API reference</h1>
  <p class="lede">Automate projects, tasks, runs, approvals and reporting. Every endpoint below is generated from the OpenAPI contract shipped with the platform.</p>
  <div class="hero-links"><a class="primary" href="openapi.yaml" download>Download OpenAPI</a><a href="pages/api-basics.html">API basics</a><a href="pages/api-events.html">Event catalog</a></div>
</div>''']

    out.append('<section id="authentication" class="api-section"><h2>Authentication</h2>')
    out.append('<p>All endpoints except <code>/auth/*</code> and <code>/healthz</code> require a bearer token. Obtain one with the login endpoint, then send it in the <code>Authorization</code> header.</p>')
    out.append('<pre><code>TOKEN=$(curl -s -X POST http://localhost:18080/api/v1/auth/login \\\n  -H "Content-Type: application/json" \\\n  -d \'{"email":"you@company.com","password":"..."}\' | jq -r .access_token)\n\ncurl -H "Authorization: Bearer $TOKEN" \\\n  http://localhost:18080/api/v1/orgs/$ORG/projects</code></pre>')
    if schemes:
        rows = "".join(f'<tr><td><code>{esc(k)}</code></td><td>{esc(v.get("type", ""))}</td><td>{md.inline(v.get("description") or "Send Authorization: Bearer &lt;access token&gt;.")}</td></tr>' for k, v in schemes.items())
        out.append(f'<div class="table-wrap"><table><thead><tr><th>Scheme</th><th>Type</th><th>Description</th></tr></thead><tbody>{rows}</tbody></table></div>')
    else:
        out.append('<p>No security schemes are declared.</p>')
    if servers:
        out.append("<p><strong>Servers:</strong> " + ", ".join(f'<code>{esc(s.get("url"))}</code>' for s in servers) + "</p>")
    out.append('<p>Every path except <code>/auth/*</code> and <code>/healthz</code> is organization-scoped. Access tokens are short-lived; refresh tokens rotate.</p>')
    out.append('<p><strong>Base URL:</strong> <code>http://localhost:18080/api/v1</code> locally, or your deployment URL plus <code>/api/v1</code>.</p>')
    out.append('<p><strong>Conventions:</strong> cursor pagination with <code>limit</code>/<code>cursor</code>; <code>Idempotency-Key</code> on mutations; errors returned as <code>{"error":{"code","message","request_id"}}</code>; <code>429</code> includes <code>Retry-After</code>. See <a href="pages/api-basics.html">API basics</a>.</p>')
    out.append('</section>')

    out.append('<section id="operations" class="api-section"><h2>Operations</h2>')
    for tag in ops_by_tag:
        out.append(f'<div class="api-tag-summary"><a href="#tag-{slugify(tag)}">{esc(tag)}</a><span>{len(ops_by_tag[tag])}</span></div>')
    out.append('</section>')

    for tag, ops in ops_by_tag.items():
        out.append(f'<section id="tag-{slugify(tag)}" class="api-section"><div class="tag-head"><h2>{esc(tag)}</h2><span>{len(ops)} operations</span></div>')
        if tags.get(tag):
            out.append(f'<p class="muted">{md.inline(tags[tag])}</p>')
        for path, method, op in ops:
            out.append('<article class="endpoint">')
            out.append(f'<div class="endpoint-head"><span class="method {method}">{method.upper()}</span><code class="path">{esc(path)}</code>' + (f'<span class="opid">{esc(op.get("operationId",""))}</span>' if op.get("operationId") else "") + '</div>')
            if op.get("summary"):
                out.append(f'<h3>{esc(op["summary"])}</h3>')
            if op.get("description"):
                out.append(f'<p>{md.inline(op["description"])}</p>')
            params = op.get("parameters", [])
            if params:
                out.append('<div class="table-wrap"><table><thead><tr><th>Parameter</th><th>In</th><th>Required</th><th>Schema</th><th>Description</th></tr></thead><tbody>')
                for p in params:
                    out.append(f'<tr><td><code>{esc(p.get("name"))}</code></td><td>{esc(p.get("in"))}</td><td>{"yes" if p.get("required") else "no"}</td><td>{schema_link(p.get("schema", {}))}</td><td>{md.inline(p.get("description", ""))}</td></tr>')
                out.append('</tbody></table></div>')
            req = op.get("requestBody", {})
            if req:
                content = req.get("content", {})
                parts = [f'<code>{esc(ct)}</code> {schema_link(media.get("schema", {}))}' for ct, media in content.items()]
                out.append(f'<p><strong>Request body</strong>{" (required)" if req.get("required") else ""}: ' + " · ".join(parts) + "</p>")
            responses = op.get("responses", {})
            if responses:
                out.append('<div class="table-wrap"><table><thead><tr><th>Status</th><th>Description</th><th>Schema</th></tr></thead><tbody>')
                for status, resp in responses.items():
                    content = resp.get("content", {}) if isinstance(resp, dict) else {}
                    schema = next((m.get("schema", {}) for m in content.values()), {})
                    out.append(f'<tr><td><code>{esc(status)}</code></td><td>{md.inline(resp.get("description", "")) if isinstance(resp, dict) else ""}</td><td>{schema_link(schema) if schema else ""}</td></tr>')
                out.append('</tbody></table></div>')
            if op.get("security"):
                out.append('<p class="meta-line">Security: ' + ", ".join(esc(next(iter(s.keys()))) for s in op["security"] if s) + "</p>")
            out.append('</article>')
        out.append('</section>')

    schemas = spec.get("components", {}).get("schemas", {})
    out.append('<section id="schemas" class="api-section"><div class="tag-head"><h2>Schemas</h2><span>' + str(len(schemas)) + ' definitions</span></div>')
    for name, schema in schemas.items():
        required = set(schema.get("required", []))
        props = schema.get("properties", {})
        rows = []
        for pname, prop in props.items():
            rows.append(f'<tr><td><code>{esc(pname)}</code>{" *" if pname in required else ""}</td><td>{schema_link(prop)}</td><td>{md.inline(prop.get("description", ""))}</td></tr>')
        body = f'<div class="table-wrap"><table><thead><tr><th>Field</th><th>Type</th><th>Description</th></tr></thead><tbody>{"".join(rows)}</tbody></table></div>' if rows else f'<p class="muted">{esc(schema.get("description", "No object properties."))}</p>'
        out.append(f'<details class="schema" id="schema-{slugify(name)}"><summary><code>{esc(name)}</code><span>{len(props)} fields</span></summary>{body}</details>')
    out.append('</section>')

    records = load_docs()
    add_extra_pages(records)
    return page_shell("API reference", "\n".join(out), records, "api", "", f"REST API reference: {operation_count} operations and {len(schemas)} schemas, generated from OpenAPI.", toc_html, "api-page")


def add_extra_pages(records: list[dict]) -> None:
    events = ROOT / "api" / "events.md"
    if not events.exists():
        return
    text = events.read_text(encoding="utf-8")
    m = re.search(r"^#\s+(.+)$", text, re.M)
    records.append({
        "slug": "api-events",
        "file": "api/events.md",
        "source": "api/events.md",
        "title": "SSE event catalog",
        "description": "Live run events, payloads and reconnect behaviour.",
        "group": "api",
        "order": "00000002-api-events",
        "text": text,
    })
    records.sort(key=lambda r: (GROUP_ORDER.get(r["group"], 99), r["order"]))


def main():
    records = load_docs()
    add_extra_pages(records)
    if PAGES.exists():
        shutil.rmtree(PAGES)
    PAGES.mkdir(parents=True)
    for r in records:
        (PAGES / f'{r["slug"]}.html').write_text(doc_page(r, records), encoding="utf-8")
    (OUT / "index.html").write_text(index_page(records), encoding="utf-8")
    (OUT / "api.html").write_text(render_api(), encoding="utf-8")
    shutil.copy2(ROOT / "api" / "openapi.yaml", OUT / "openapi.yaml")
    search = [{"slug": r["slug"], "title": r["title"], "group": r["group"],
               "text": re.sub(r"\s+", " ", r["text"])[:4000]} for r in records]
    (OUT / "search-index.json").write_text(json.dumps(search, ensure_ascii=False), encoding="utf-8")
    print(f"user docs: {len(records)} pages, {len(list(PAGES.glob('*.html')))} generated")


if __name__ == "__main__":
    main()
