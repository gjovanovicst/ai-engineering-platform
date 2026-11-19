"use client";

import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { SidePanel } from "@/components/ui/side-panel";
import { assistantStream, type AssistantEvent } from "@/lib/api";

type ChatMessage = { role: "user" | "assistant" | "note"; text: string };

const suggestions = [
  "Explain the last failed run and what to do next.",
  "Draft a task with acceptance criteria for a health endpoint.",
  "What needs my attention in this workspace today?",
];

/** The assistant side panel: the page context, the streamed answer and the
 * confirmation before a write. */
export function AssistantPanel({ orgID, onClose }: { orgID: string; onClose: () => void }) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [prompt, setPrompt] = useState("");
  const [conversationID, setConversationID] = useState("");
  const [pendingTool, setPendingTool] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const endRef = useRef<HTMLDivElement | null>(null);
  const context = typeof window === "undefined" ? "" : window.location.pathname;

  useEffect(() => {
    endRef.current?.scrollIntoView?.({ block: "end" });
  }, [messages, busy]);

  async function ask(text: string, confirm = false) {
    if (busy) return;
    const question = text.trim();
    if (!question) return;
    setBusy(true);
    setError(null);
    setPendingTool(null);
    if (!confirm) setMessages((current) => [...current, { role: "user", text: question }]);
    try {
      await assistantStream(
        orgID,
        { conversation_id: conversationID || undefined, prompt: question, confirm },
        (event: AssistantEvent) => {
          if (event.conversation_id) setConversationID(event.conversation_id);
          if (event.type === "text") setMessages((current) => [...current, { role: "assistant", text: event.text ?? "" }]);
          if (event.type === "pending") {
            setMessages((current) => [...current, { role: "assistant", text: event.text ?? "" }]);
            setPendingTool(event.pending_tool ?? "");
          }
        },
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "The assistant could not answer");
    } finally {
      setBusy(false);
    }
  }

  function confirmPending() {
    if (!pendingTool) return;
    setMessages((current) => [...current, { role: "note", text: "You confirmed the action." }]);
    void ask(`Yes, run ${pendingTool} now.`, true);
  }

  return (
    <SidePanel
      size="lg"
      title="Assistant"
      description={`Ask about this workspace. Context: ${context || "/"}`}
      onClose={onClose}
      footer={
        <form
          className="flex w-full items-center gap-2"
          onSubmit={(event) => {
            event.preventDefault();
            const text = prompt;
            setPrompt("");
            void ask(text);
          }}
        >
          <input
            aria-label="Ask the assistant"
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            placeholder="Ask about a run, a task or what to do next…"
            className="h-9 flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm"
          />
          <Button type="submit" loading={busy}>Send</Button>
        </form>
      }
    >
      <div className="space-y-3 p-5 text-sm">
        {messages.length === 0 ? (
          <div>
            <p className="text-xs text-text-secondary">
              The assistant reads your projects, tasks and runs. It never approves a pull request or merges; writes need your confirmation.
            </p>
            <div className="mt-3 space-y-2">
              {suggestions.map((suggestion) => (
                <button
                  key={suggestion}
                  type="button"
                  onClick={() => void ask(suggestion)}
                  className="block w-full rounded-md border border-border bg-bg-subtle px-3 py-2 text-left text-xs text-text-secondary hover:bg-bg-hover"
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : null}
        {messages.map((message, index) => (
          <div
            key={`${message.role}-${index}`}
            className={`rounded-lg px-3 py-2 ${
              message.role === "user"
                ? "ml-8 bg-brand/10 text-text-primary"
                : message.role === "note"
                  ? "bg-bg-subtle text-xs text-text-muted"
                  : "mr-8 border border-border bg-bg-surface text-text-secondary"
            }`}
          >
            {message.text}
          </div>
        ))}
        {busy ? <p className="text-xs text-text-muted">The assistant is thinking…</p> : null}
        {pendingTool ? (
          <div className="rounded-lg border border-status-warning/40 bg-status-warning/10 p-3 text-xs">
            <p className="font-medium">Confirmation needed</p>
            <p className="mt-1 text-text-secondary">
              The assistant wants to run <code>{pendingTool}</code>. Nothing is changed until you confirm.
            </p>
            <div className="mt-2 flex gap-2">
              <Button className="h-8 text-xs" loading={busy} onClick={confirmPending}>Confirm</Button>
              <Button variant="secondary" className="h-8 text-xs" onClick={() => setPendingTool(null)}>Cancel</Button>
            </div>
          </div>
        ) : null}
        {error ? <p role="alert" className="text-xs text-status-danger">{error}</p> : null}
        <div ref={endRef} />
      </div>
    </SidePanel>
  );
}
