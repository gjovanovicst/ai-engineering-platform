"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { commandSchema } from "@/lib/schemas";

/** A one-line command setting with inline validation and unsaved-state reset. */
export function CommandForm({
  id,
  title,
  description,
  placeholder,
  initial,
  onSave,
  children,
}: {
  id: string;
  title: string;
  description: string;
  placeholder?: string;
  initial: string;
  onSave: (command: string) => Promise<unknown>;
  children?: React.ReactNode;
}) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<{ command: string }>(commandSchema, { defaultValues: { command: initial } });
  const [serverError, setServerError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    reset({ command: initial });
  }, [initial, reset]);

  const submit = handleSubmit(async ({ command }) => {
    setBusy(true);
    setServerError(null);
    try {
      await onSave(command);
      reset({ command });
    } catch (err) {
      setServerError(err instanceof ApiError ? err.message : "Could not save the command");
    } finally {
      setBusy(false);
    }
  });

  return (
    <section className="mt-5 rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">{title}</h2>
      <p className="mt-1 text-xs text-text-secondary">{description}</p>
      <form onSubmit={submit} className="mt-2">
        <Field label={title} htmlFor={id} error={errors.command?.message}>
          <div className="flex flex-wrap items-end gap-2">
            <Input
              id={id}
              aria-invalid={Boolean(errors.command)}
              className="h-9 min-w-[240px] flex-1 font-mono text-xs"
              {...register("command")}
              placeholder={placeholder}
            />
            <Button type="submit" variant="secondary" loading={busy}>
              Save
            </Button>
          </div>
        </Field>
        {serverError ? (
          <p role="alert" className="mt-2 text-xs text-status-danger">
            {serverError}
          </p>
        ) : null}
      </form>
      {children}
    </section>
  );
}
