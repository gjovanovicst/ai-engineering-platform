import { z } from "zod";

export const acceptanceSchema = z.object({
  given: z.string(),
  when: z.string(),
  then: z.string(),
});

/** One-line command fields (the test, e2e and visual commands). */
export const commandSchema = z.object({
  command: z
    .string()
    .trim()
    .max(500, "At most 500 characters.")
    .refine((value) => !value.includes("\n") && !value.includes("\r"), "The command must be one line."),
});

export type CommandValues = z.infer<typeof commandSchema>;

/** The task composer schema; the ready gating is checked by the model. */
export const taskComposerSchema = z.object({
  title: z.string().trim().min(1, "A title is required.").max(200, "The title is longer than 200 characters."),
  description: z.string().max(20000),
  references: z.string(),
  priority: z.enum(["low", "med", "high"]),
  tags: z.string(),
  acceptance: z.array(acceptanceSchema).min(1, "Keep at least one acceptance criterion."),
  files: z.string(),
  outOfScope: z.string(),
  testPlan: z.string(),
  rollback: z.string(),
  dependsOn: z.array(z.string()),
});

export type TaskComposerValues = z.infer<typeof taskComposerSchema>;

export const componentStatuses = ["approved", "draft", "published", "deprecated", "forbidden"] as const;
export const componentCategories = ["layout", "navigation", "data", "form", "feedback", "overlay", "typography", "other"] as const;
export const tokenCategories = ["color", "typography", "spacing", "radius", "layout", "other"] as const;

/** The component registry entry (the add/edit form). */
export const componentEntrySchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  status: z.enum(componentStatuses),
  category: z.enum(componentCategories),
  purpose: z.string().max(500, "At most 500 characters."),
  rules: z.string().max(2000, "At most 2000 characters."),
  framework: z.string().max(40, "At most 40 characters."),
  package: z.string().max(200, "At most 200 characters."),
  import_path: z.string().max(300, "At most 300 characters."),
  version: z.string().max(40, "At most 40 characters."),
  docs: z.string().max(500, "At most 500 characters."),
  example: z.string().max(2000, "At most 2000 characters."),
});

export type ComponentEntryValues = z.infer<typeof componentEntrySchema>;

/** A design token (the add form). */
export const designTokenSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  value: z.string().trim().min(1, "A value is required.").max(200, "At most 200 characters."),
  category: z.enum(tokenCategories),
});

export type DesignTokenValues = z.infer<typeof designTokenSchema>;
