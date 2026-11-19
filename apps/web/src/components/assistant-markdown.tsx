import type { ReactNode } from "react";

/** Minimal markdown for assistant answers: headings, lists, bold, inline code
 * and fenced code. Keeps the bundle dependency-free. */
export function AssistantMarkdown({ text }: { text: string }) {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let list: string[] = [];
  let code: string[] | null = null;

  const flushList = () => {
    if (list.length === 0) return;
    blocks.push(
      <ul key={`ul-${blocks.length}`} className="my-2 list-disc space-y-1 pl-5">
        {list.map((item, index) => (
          <li key={index}>{inline(item)}</li>
        ))}
      </ul>,
    );
    list = [];
  };

  for (const raw of lines) {
    const line = raw.trimEnd();
    if (line.startsWith("```")) {
      if (code) {
        blocks.push(
          <pre key={`code-${blocks.length}`} className="my-2 overflow-x-auto rounded-md bg-bg-subtle p-3 text-xs">
            <code>{code.join("\n")}</code>
          </pre>,
        );
        code = null;
      } else {
        flushList();
        code = [];
      }
      continue;
    }
    if (code) {
      code.push(raw);
      continue;
    }
    if (line.startsWith("- ") || line.startsWith("* ")) {
      list.push(line.slice(2));
      continue;
    }
    const numbered = line.match(/^\d+\.\s+(.*)$/);
    if (numbered) {
      list.push(numbered[1]);
      continue;
    }
    flushList();
    if (line === "") continue;
    const heading = line.match(/^(#{1,4})\s+(.*)$/);
    if (heading) {
      const level = heading[1].length;
      const size = level <= 2 ? "text-sm font-semibold" : "text-xs font-semibold";
      blocks.push(
        <p key={`h-${blocks.length}`} className={`mt-3 mb-1 ${size}`}>
          {inline(heading[2])}
        </p>,
      );
      continue;
    }
    if (line.startsWith("> ")) {
      blocks.push(
        <blockquote key={`q-${blocks.length}`} className="my-2 border-l-2 border-border pl-3 text-text-muted">
          {inline(line.slice(2))}
        </blockquote>,
      );
      continue;
    }
    blocks.push(
      <p key={`p-${blocks.length}`} className="my-1 leading-relaxed">
        {inline(line)}
      </p>,
    );
  }
  flushList();
  return <div className="space-y-1">{blocks}</div>;
}

/** Bold and inline code inside one line. */
function inline(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let last = 0;
  let match: RegExpExecArray | null;
  let key = 0;
  while ((match = pattern.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const token = match[0];
    if (token.startsWith("**")) {
      parts.push(<strong key={key++}>{token.slice(2, -2)}</strong>);
    } else {
      parts.push(
        <code key={key++} className="rounded bg-bg-subtle px-1 py-0.5 font-mono text-[0.85em]">
          {token.slice(1, -1)}
        </code>,
      );
    }
    last = match.index + token.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}
