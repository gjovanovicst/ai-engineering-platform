"use client";

import { useState } from "react";

import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { HelpTip } from "@/components/ui/help-tip";
import { Input } from "@/components/ui/input";
import { Modal } from "@/components/ui/modal";
import { PromptDialog } from "@/components/ui/prompt-dialog";
import { SidePanel } from "@/components/ui/side-panel";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { Tabs } from "@/components/ui/tabs";

function TabsExample() {
  const [tab, setTab] = useState("one");
  return (
    <Tabs ariaLabel="Example tabs" value={tab} onChange={setTab} items={[
      { id: "one", label: "Overview" },
      { id: "two", label: "Details", count: 3 },
    ]} />
  );
}

function ModalExample() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>Open the modal</Button>
      <Modal open={open} onClose={() => setOpen(false)} title="Example modal">
        <p className="text-sm text-text-secondary">A small centred dialog.</p>
      </Modal>
    </>
  );
}

function SidePanelExample() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>Open the drawer</Button>
      <SidePanel title="Example drawer" description="The form lives here." onClose={() => setOpen(false)} dirty>
        <Field label="Title" htmlFor="example-title"><Input id="example-title" placeholder="A title" /></Field>
      </SidePanel>
    </>
  );
}

function ConfirmExample() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="danger" onClick={() => setOpen(true)}>Delete the item</Button>
      <ConfirmDialog open={open} title="Delete the item?" message="This cannot be undone."
        confirmLabel="Delete" onConfirm={() => setOpen(false)} onCancel={() => setOpen(false)} />
    </>
  );
}

function PromptExample() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>Ask for a name</Button>
      <PromptDialog open={open} title="Name the view" message="It appears in the sidebar." label="Name"
        placeholder="e.g. Blocked runs" required onConfirm={() => setOpen(false)} onCancel={() => setOpen(false)} />
    </>
  );
}

function FieldExample() {
  const [value, setValue] = useState("");
  return (
    <div className="max-w-sm">
      <Field label="Title" htmlFor="example-field" error={value === "" ? "The title is required." : undefined}>
        <Input id="example-field" value={value} onChange={(event) => setValue(event.target.value)} aria-invalid={value === ""} />
      </Field>
    </div>
  );
}

export const playgroundExamples: Record<string, () => React.ReactNode> = {
  Button: () => (
    <div className="flex flex-wrap items-center gap-2">
      <Button>Primary</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="ghost">Ghost</Button>
      <Button variant="danger">Danger</Button>
      <Button loading>Loading</Button>
      <Button disabled>Disabled</Button>
    </div>
  ),
  Input: () => (
    <div className="flex max-w-sm flex-col gap-2">
      <Input placeholder="A plain input" />
      <Input placeholder="An invalid input" aria-invalid defaultValue="nope" />
    </div>
  ),
  Field: FieldExample,
  Alert: () => (
    <div className="flex flex-col gap-2">
      <Alert variant="info" title="Information">A neutral message.</Alert>
      <Alert variant="success" title="Done">It worked.</Alert>
      <Alert variant="warning" title="Careful">Check the setting.</Alert>
      <Alert variant="danger" title="Failed">The run failed.</Alert>
    </div>
  ),
  EmptyState: () => (
    <EmptyState title="No runs yet" description="Start a run from a ready task."
      action={<Button>Start a run</Button>} />
  ),
  Tabs: TabsExample,
  Skeleton: () => (
    <div className="max-w-sm space-y-2">
      <Skeleton className="h-9 w-full" />
      <SkeletonText lines={2} />
    </div>
  ),
  HelpTip: () => <p className="text-sm text-text-secondary">A run is a <HelpTip term="run" /> of the agent.</p>,
  Modal: ModalExample,
  SidePanel: SidePanelExample,
  ConfirmDialog: ConfirmExample,
  PromptDialog: PromptExample,
};
