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
