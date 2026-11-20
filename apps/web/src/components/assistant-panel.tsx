"use client";

import { useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { AssistantMarkdown } from "@/components/assistant-markdown";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { SidePanel } from "@/components/ui/side-panel";
import { assistantStream, clearAssistantConversations, deleteAssistantConversation, listAssistantConversations, listAssistantMessages, type AssistantConversation, type AssistantEvent } from "@/lib/api";
import { assistantCommands, expandCommand } from "@/lib/assistant-commands";

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
  const queryClient = useQueryClient();
  const [conversations, setConversations] = useState<AssistantConversation[]>([]);
  const [commandIndex, setCommandIndex] = useState(0);
  const [confirmClear, setConfirmClear] = useState(false);
  const endRef = useRef<HTMLDivElement | null>(null);
  const context = typeof window === "undefined" ? "" : window.location.pathname;
  const matchingCommands = prompt.startsWith("/") && !prompt.includes(" ")
    ? assistantCommands.filter((command) => command.name.startsWith(prompt.trim()))
    : [];

  useEffect(() => {
    endRef.current?.scrollIntoView?.({ block: "end" });
  }, [messages, busy]);

  useEffect(() => {
    listAssistantConversations(orgID)
      .then((result) => setConversations(result.data))
      .catch(() => setConversations([]));
  }, [orgID, conversationID]);

  async function openConversation(id: string) {
    setConversationID(id);
    setPendingTool(null);
    setError(null);
    try {
      const result = await listAssistantMessages(orgID, id);
      setMessages(result.data.filter((message) => message.role !== "tool").map((message) => ({ role: message.role === "user" ? "user" : "assistant", text: message.content })));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load the conversation");
    }
  }

  function newChat() {
    setConversationID("");
    setMessages([]);
    setPendingTool(null);
    setError(null);
  }

  async function removeConversation(id: string) {
    try {
      await deleteAssistantConversation(orgID, id);
      setConversations((current) => current.filter((conversation) => conversation.id !== id));
      if (conversationID === id) newChat();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not delete the conversation");
    }
  }

  function clearHistory() {
    setConfirmClear(true);
  }

  async function doClearHistory() {
    setConfirmClear(false);
    try {
      await clearAssistantConversations(orgID);
      setConversations([]);
      newChat();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not clear the conversations");
    }
  }

  async function ask(text: string, confirm = false) {
    if (busy) return;
    const original = text.trim();
    if (!original) return;
    const expanded = expandCommand(original);
    if (expanded?.command === "/help") {
      setMessages((current) => [
        ...current,
        { role: "user", text: original },
        { role: "assistant", text: assistantCommands.map((command) => `**${command.name}** — ${command.description}\n\`${command.usage}\``).join("\n") },
      ]);
      return;
    }
    const question = expanded ? expanded.prompt : original;
    setBusy(true);
    setError(null);
    setPendingTool(null);
    if (!confirm) setMessages((current) => [...current, { role: "user", text: original }]);
    try {
      await assistantStream(
        orgID,
        { conversation_id: conversationID || undefined, prompt: question, confirm },
        (event: AssistantEvent) => {
          if (event.conversation_id) setConversationID(event.conversation_id);
          if (event.type === "text") setMessages((current) => [...current, { role: "assistant", text: event.text ?? "" }]);
          if (event.type === "pending") {
            setMessages((current) => [...current, { role: "assistant", text: event.text ?? "" }]);
            setPendingTool(event.pending_tool ?? event.tool ?? "");
          }
        },
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "The assistant could not answer");
    } finally {
      setBusy(false);
      if (confirm) {
        // A confirmed write (create a task/project, retry, cancel) changes the
        // page's data: refresh every query so the list reflects it at once.
        await queryClient.invalidateQueries();
      }
    }
  }

  function confirmPending() {
    if (!pendingTool) return;
    setMessages((current) => [...current, { role: "note", text: "You confirmed the action." }]);
    void ask(`Yes, run ${pendingTool} now.`, true);
  }

  return (
    <>
    <ConfirmDialog
      open={confirmClear}
      title="Delete all earlier conversations?"
      message="Every conversation in the history is removed. This cannot be undone."
      confirmLabel="Delete all"
      cancelLabel="Keep"
      onConfirm={() => void doClearHistory()}
      onCancel={() => setConfirmClear(false)}
    />
    <SidePanel
      size="lg"
      z={130}
      assistant={false}
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
          <div className="relative flex-1">
            <input
              aria-label="Ask the assistant"
              value={prompt}
              onChange={(event) => {
                setPrompt(event.target.value);
                setCommandIndex(0);
              }}
              onKeyDown={(event) => {
                if (matchingCommands.length === 0) return;
                if (event.key === "ArrowDown") {
                  event.preventDefault();
                  setCommandIndex((index) => (index + 1) % matchingCommands.length);
                } else if (event.key === "ArrowUp") {
                  event.preventDefault();
                  setCommandIndex((index) => (index - 1 + matchingCommands.length) % matchingCommands.length);
                } else if (event.key === "Enter" && matchingCommands[commandIndex]) {
                  event.preventDefault();
                  setPrompt(`${matchingCommands[commandIndex].name} `);
                  setCommandIndex(0);
                }
              }}
              placeholder="Ask, or type / for commands…"
              className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
            />
            {matchingCommands.length > 0 ? (
              <ul role="listbox" aria-label="Commands" className="absolute bottom-full left-0 z-10 mb-1 w-full rounded-md border border-border bg-bg-surface p-1 text-xs shadow-lg">
                {matchingCommands
                  .map((command, index) => (
                    <li key={command.name}>
                      <button
                        type="button"
                        role="option"
                        aria-selected={index === commandIndex}
                        onMouseEnter={() => setCommandIndex(index)}
                        onClick={() => setPrompt(command.name + " ")}
                        className={`flex w-full items-center justify-between gap-2 rounded px-2 py-1.5 text-left ${index === commandIndex ? "bg-bg-hover" : ""}`}
                      >
                        <span className="font-mono">{command.name}</span>
                        <span className="text-text-muted">{command.description}</span>
                      </button>
                    </li>
                  ))}
              </ul>
            ) : null}
          </div>
          <Button type="submit" loading={busy}>Send</Button>
        </form>
      }
    >
      <div className="flex min-h-0 flex-1">
        <aside className="w-48 shrink-0 space-y-1 border-r border-border p-2 text-xs">
          <button type="button" onClick={newChat} className="w-full rounded-md border border-border px-2 py-1.5 text-left text-text-secondary hover:bg-bg-hover">
            + New chat
          </button>
          {conversations.length === 0 ? <p className="px-2 py-1 text-text-muted">No earlier conversations.</p> : null}
          {conversations.map((conversation) => (
            <div key={conversation.id} className="group flex items-center gap-1">
              <button
                type="button"
                onClick={() => void openConversation(conversation.id)}
                className={`min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-left hover:bg-bg-hover ${conversation.id === conversationID ? "bg-bg-hover text-text-primary" : "text-text-secondary"}`}
                title={conversation.title}
              >
                {conversation.title || "Untitled"}
              </button>
              <button
                type="button"
                onClick={() => void removeConversation(conversation.id)}
                aria-label={`Delete ${conversation.title || "conversation"}`}
                className="shrink-0 rounded px-1 text-text-muted opacity-0 hover:text-status-danger group-hover:opacity-100"
              >
                ×
              </button>
            </div>
          ))}
          {conversations.length > 0 ? (
            <button type="button" onClick={() => void clearHistory()} className="mt-2 w-full rounded-md px-2 py-1.5 text-left text-status-danger hover:bg-bg-hover">
              Clear all history
            </button>
          ) : null}
        </aside>
        <div className="min-w-0 flex-1 space-y-3 overflow-y-auto p-5 text-sm">
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
            {message.role === "assistant" ? <AssistantMarkdown text={message.text} /> : message.text}
          </div>
        ))}
        {busy ? <p className="text-xs text-text-muted">The assistant is thinking…</p> : null}
        {pendingTool ? (
          <div className="sticky bottom-0 rounded-lg border border-status-warning/40 bg-bg-surface p-3 text-xs shadow-lg">
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
      </div>
    </SidePanel>
    </>
  );
}
