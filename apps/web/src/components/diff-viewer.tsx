type LineKind = "meta" | "hunk" | "add" | "del" | "ctx";

function kindOf(line: string): LineKind {
  if (line.startsWith("+++") || line.startsWith("---") || line.startsWith("diff --git") || line.startsWith("index ")) {
    return "meta";
  }
  if (line.startsWith("@@")) return "hunk";
  if (line.startsWith("+")) return "add";
  if (line.startsWith("-")) return "del";
  return "ctx";
}

const styles: Record<LineKind, string> = {
  meta: "text-text-muted",
  hunk: "bg-ai-subtle/40 text-ai",
  add: "bg-status-success/10 text-status-success",
  del: "bg-status-danger/10 text-status-danger",
  ctx: "text-text-secondary",
};

export function DiffViewer({ diff }: { diff: string }) {
  const lines = diff.replace(/\n$/, "").split("\n");
  const added = lines.filter((line) => kindOf(line) === "add").length;
  const removed = lines.filter((line) => kindOf(line) === "del").length;

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="flex items-center justify-between border-b border-border px-4 py-3">
        <h2 className="text-sm font-semibold">Changes</h2>
        <span className="font-mono text-xs">
          <span className="text-status-success">+{added}</span>{" "}
          <span className="text-status-danger">-{removed}</span>
        </span>
      </header>
      <pre className="max-h-[420px] overflow-auto p-4 font-mono text-xs leading-relaxed">
        {lines.map((line, index) => (
          <div key={index} className={`whitespace-pre px-2 ${styles[kindOf(line)]}`}>
            {line || " "}
          </div>
        ))}
      </pre>
    </section>
  );
}
