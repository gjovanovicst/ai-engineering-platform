"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm, type FieldValues, type Resolver, type UseFormProps } from "react-hook-form";
import type { z } from "zod";

/** A form bound to a zod schema: validation runs on blur and on submit. */
export function useZodForm<Values extends FieldValues>(
  schema: z.ZodType<Values>,
  options?: Omit<UseFormProps<Values>, "resolver">,
) {
  // The zod v4 resolver is typed against its own internal schema type; the
  // runtime contract (a schema whose output is Values) holds.
  const resolver = zodResolver(schema as never) as unknown as Resolver<Values>;
  return useForm<Values>({ resolver, mode: "onBlur", ...options });
}

/** Warns the browser before closing the tab with unsaved changes. */
export function useUnsavedChanges(dirty: boolean) {
  useEffect(() => {
    if (!dirty) return;
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);
}
