"use client";

export type DiffRow = { type: "same" | "add" | "del"; text: string };

// lineDiff is a plain LCS line diff: enough for rule bodies (bounded size).
export function lineDiff(left: string, right: string): DiffRow[] {
  const a = left.split("\n");
  const b = right.split("\n");
  if (a.length * b.length > 4_000_000) {
    return [
      ...a.map((text) => ({ type: "del" as const, text })),
      ...b.map((text) => ({ type: "add" as const, text })),
    ];
  }
  const dp: number[][] = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i--) {
    for (let j = b.length - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const rows: DiffRow[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      rows.push({ type: "same", text: a[i] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      rows.push({ type: "del", text: a[i] });
      i++;
    } else {
      rows.push({ type: "add", text: b[j] });
      j++;
    }
  }
  while (i < a.length) rows.push({ type: "del", text: a[i++] });
  while (j < b.length) rows.push({ type: "add", text: b[j++] });
  return rows;
}

export function RuleDiff({ left, right, leftLabel, rightLabel }: {
  left: string;
  right: string;
  leftLabel: string;
  rightLabel: string;
}) {
  const rows = lineDiff(left, right);
  const added = rows.filter((row) => row.type === "add").length;
  const removed = rows.filter((row) => row.type === "del").length;
  return (
    <section className="border-t border-border">
      <header className="flex flex-wrap items-center gap-2 px-4 py-2 text-xs text-text-secondary">
        <span className="font-medium text-text-primary">
          {leftLabel} → {rightLabel}
        </span>
        <span className="text-status-success">+{added}</span>
        <span className="text-status-danger">−{removed}</span>
      </header>
      <ul className="max-h-72 overflow-auto font-mono text-[11px] leading-5">
        {rows.map((row, index) => (
          <li
            key={`${index}-${row.type}`}
            className={
              row.type === "add"
                ? "bg-status-success/10 px-4 text-status-success"
                : row.type === "del"
                  ? "bg-status-danger/10 px-4 text-status-danger"
                  : "px-4 text-text-secondary"
            }
          >
            <span aria-hidden className="mr-2 inline-block w-3">
              {row.type === "add" ? "+" : row.type === "del" ? "-" : " "}
            </span>
            {row.text || " "}
          </li>
        ))}
      </ul>
    </section>
  );
}
