"use client";

import { useState, type ReactNode } from "react";

/** A focused markdown renderer for assistant answers: headings, lists (nested),
 * tables, bold/italic, inline code, fenced code with copy, links and quotes.
 * Dependency-free so the panel stays light. */
export function AssistantMarkdown({ text }: { text: string }) {
  return <div className="space-y-1.5">{renderBlocks(text.replace(/\r\n/g, "\n").split("\n"))}</div>;
}

type Block = { kind: "p" | "h" | "quote"; text: string } | { kind: "ul" | "ol"; items: string[] } | { kind: "table"; rows: string[][] } | { kind: "code"; lang: string; lines: string[] };

function parse(lines: string[]): Block[] {
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.trim().startsWith("```")) {
      const lang = line.trim().slice(3);
      const body: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) body.push(lines[i++]);
      i++;
      blocks.push({ kind: "code", lang, lines: body });
      continue;
    }
    if (line.trim() === "") {
      i++;
      continue;
    }
    const heading = line.match(/^(#{1,4})\s+(.*)$/);
    if (heading) {
      blocks.push({ kind: "h", text: `${heading[1].length}|${heading[2]}` });
      i++;
      continue;
    }
    if (/^\s*\|.*\|\s*$/.test(line) && /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1] ?? "")) {
      const rows: string[][] = [splitRow(line)];
      i += 2;
      while (i < lines.length && /^\s*\|.*\|\s*$/.test(lines[i])) {
        rows.push(splitRow(lines[i]));
        i++;
      }
      blocks.push({ kind: "table", rows });
      continue;
    }
    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) items.push(lines[i++].replace(/^\s*[-*]\s+/, ""));
      blocks.push({ kind: "ul", items });
      continue;
    }
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) items.push(lines[i++].replace(/^\s*\d+\.\s+/, ""));
      blocks.push({ kind: "ol", items });
      continue;
    }
    if (line.startsWith("> ")) {
      const parts: string[] = [];
      while (i < lines.length && lines[i].startsWith("> ")) parts.push(lines[i++].slice(2));
      blocks.push({ kind: "quote", text: parts.join(" ") });
      continue;
    }
    blocks.push({ kind: "p", text: line });
    i++;
  }
  return blocks;
}

function splitRow(line: string): string[] {
  return line.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((cell) => cell.trim());
}

function renderBlocks(lines: string[]): ReactNode[] {
  return parse(lines).map((block, index) => {
    switch (block.kind) {
      case "code":
        return <CodeBlock key={index} lang={block.lang} code={block.lines.join("\n")} />;
      case "table":
        return (
          <div key={index} className="my-2 overflow-x-auto rounded-md border border-border">
            <table className="w-full text-xs">
              <thead className="bg-bg-subtle text-left text-text-muted">
                <tr>{block.rows[0].map((cell, cellIndex) => <th key={cellIndex} className="px-2 py-1.5 font-medium">{inline(cell)}</th>)}</tr>
              </thead>
              <tbody>
                {block.rows.slice(1).map((row, rowIndex) => (
                  <tr key={rowIndex} className="border-t border-border">{row.map((cell, cellIndex) => <td key={cellIndex} className="px-2 py-1.5">{inline(cell)}</td>)}</tr>
                ))}
              </tbody>
            </table>
          </div>
        );
      case "ul":
        return (
          <ul key={index} className="my-1.5 list-disc space-y-1 pl-5">
            {block.items.map((item, itemIndex) => <li key={itemIndex} className="leading-relaxed">{inline(item)}</li>)}
          </ul>
        );
      case "ol":
        return (
          <ol key={index} className="my-1.5 list-decimal space-y-1 pl-5">
            {block.items.map((item, itemIndex) => <li key={itemIndex} className="leading-relaxed">{inline(item)}</li>)}
          </ol>
        );
      case "quote":
        return <blockquote key={index} className="my-1.5 border-l-2 border-brand/50 pl-3 text-text-muted">{inline(block.text)}</blockquote>;
      case "h": {
        const [levelText, ...rest] = block.text.split("|");
        const level = Number(levelText);
        const size = level <= 2 ? "text-sm font-semibold mt-3" : "text-xs font-semibold uppercase tracking-wide text-text-muted mt-2";
        return <p key={index} className={`mb-0.5 ${size}`}>{inline(rest.join("|"))}</p>;
      }
      default:
        return <p key={index} className="leading-relaxed">{inline(block.text)}</p>;
    }
  });
}

function CodeBlock({ lang, code }: { lang: string; code: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="my-2 overflow-hidden rounded-md border border-border">
      <div className="flex items-center justify-between bg-bg-subtle px-2 py-1 text-[11px] text-text-muted">
        <span className="font-mono">{lang || "code"}</span>
        <button
          type="button"
          onClick={() => {
            void navigator.clipboard?.writeText(code).then(() => {
              setCopied(true);
              setTimeout(() => setCopied(false), 1500);
            });
          }}
          className="rounded px-1.5 py-0.5 hover:bg-bg-hover"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <pre className="overflow-x-auto bg-bg-surface p-2.5 text-[11px] leading-relaxed"><code>{code}</code></pre>
    </div>
  );
}

/** Bold, italic, inline code and links inside one line. */
function inline(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`|\[[^\]]+\]\([^)]+\))/g;
  let last = 0;
  let match: RegExpExecArray | null;
  let key = 0;
  while ((match = pattern.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const token = match[0];
    if (token.startsWith("**")) {
      parts.push(<strong key={key++} className="font-semibold text-text-primary">{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("`")) {
      parts.push(<code key={key++} className="rounded bg-bg-subtle px-1 py-0.5 font-mono text-[0.85em]">{token.slice(1, -1)}</code>);
    } else if (token.startsWith("[")) {
      const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      if (link) {
        parts.push(<a key={key++} href={link[2]} target="_blank" rel="noreferrer" className="text-brand underline">{link[1]}</a>);
      } else {
        parts.push(token);
      }
    } else {
      parts.push(<em key={key++}>{token.slice(1, -1)}</em>);
    }
    last = match.index + token.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}
