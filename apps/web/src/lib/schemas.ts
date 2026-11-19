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

const emailField = z.email("Enter a valid email address.");
const passwordField = z
  .string()
  .min(12, "At least 12 characters.")
  .max(128, "At most 128 characters.");

export const loginSchema = z.object({
  email: emailField,
  password: z.string().min(1, "A password is required."),
});
export type LoginValues = z.infer<typeof loginSchema>;

export const registerSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(120, "At most 120 characters."),
  email: emailField,
  org_name: z.string().trim().max(80, "At most 80 characters."),
  password: passwordField,
});
export type RegisterValues = z.infer<typeof registerSchema>;

export const acceptInviteSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(120, "At most 120 characters."),
  password: passwordField,
});
export type AcceptInviteValues = z.infer<typeof acceptInviteSchema>;

export const searchSchema = z.object({
  q: z.string().trim().min(2, "Type at least two characters."),
});
export type SearchValues = z.infer<typeof searchSchema>;

export const projectSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  description: z.string().max(2000, "At most 2000 characters."),
});
export type ProjectValues = z.infer<typeof projectSchema>;

export const repoConnectSchema = z.object({
  provider: z.enum(["github", "gitlab"]),
  name: z.string().trim().min(1, "A repository is required.").max(200, "At most 200 characters."),
  default_branch: z.string().trim().max(200, "At most 200 characters."),
  webhook_secret: z.string().max(200, "At most 200 characters."),
});
export type RepoConnectValues = z.infer<typeof repoConnectSchema>;

export const aiProviderSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  base_url: z.string().trim().min(1, "A base URL is required.").max(300, "At most 300 characters."),
  model: z.string().trim().max(80, "At most 80 characters."),
  api_key: z.string().max(400, "At most 400 characters."),
});
export type AiProviderValues = z.infer<typeof aiProviderSchema>;

export const modelPriceSchema = z.object({
  provider: z.string().trim().max(80, "At most 80 characters."),
  model: z.string().trim().min(1, "A model is required.").max(80, "At most 80 characters."),
  input: z.string().max(40, "At most 40 characters."),
  output: z.string().max(40, "At most 40 characters."),
});
export type ModelPriceValues = z.infer<typeof modelPriceSchema>;

export const repoCredentialSchema = z.object({
  token: z.string().trim().min(1, "A token is required.").max(500, "At most 500 characters."),
});
export type RepoCredentialValues = z.infer<typeof repoCredentialSchema>;

export const environmentSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  url: z.string().trim().min(1, "A URL is required.").max(300, "At most 300 characters."),
});
export type EnvironmentValues = z.infer<typeof environmentSchema>;

export const inviteSchema = z.object({
  email: z.email("Enter a valid email address."),
  role: z.enum(["admin", "designer", "member", "viewer"]),
});
export type InviteValues = z.infer<typeof inviteSchema>;

export const apiKeySchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  role: z.enum(["admin", "member", "viewer"]),
  days: z
    .string()
    .trim()
    .refine((value) => value === "" || (/^\d+$/.test(value) && Number(value) >= 0 && Number(value) <= 3650), "A whole number of days between 0 and 3650."),
});
export type ApiKeyValues = z.infer<typeof apiKeySchema>;

export const ruleSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(120, "At most 120 characters."),
  level: z.enum(["organization", "project", "module", "task"]),
  body: z.string().trim().min(1, "The body is required.").max(20000, "At most 20000 characters."),
});
export type RuleValues = z.infer<typeof ruleSchema>;

export const agentPluginSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
  version: z.string().trim().min(1, "A version is required.").max(40, "At most 40 characters."),
  image: z.string().trim().min(1, "An image is required.").max(300, "At most 300 characters."),
  command: z.string().trim().min(1, "A command is required.").max(500, "At most 500 characters."),
  types: z.array(z.string()).min(1, "Choose at least one agent type."),
});
export type AgentPluginValues = z.infer<typeof agentPluginSchema>;

export const designStandardSchema = z.object({
  name: z.string().trim().min(3, "The name needs at least 3 characters.").max(80, "At most 80 characters."),
  accessibility: z.string().max(4000, "At most 4000 characters."),
  responsive: z.string().max(4000, "At most 4000 characters."),
  patterns: z.string().max(4000, "At most 4000 characters."),
  advanced: z
    .string()
    .max(8000, "At most 8000 characters.")
    .refine((value) => {
      if (!value.trim()) return true;
      try {
        JSON.parse(value);
        return true;
      } catch {
        return false;
      }
    }, "The advanced JSON must be valid JSON."),
});
export type DesignStandardValues = z.infer<typeof designStandardSchema>;

export const previewSchema = z.object({
  image: z.string().trim().min(1, "An image is required.").max(300, "At most 300 characters."),
  port: z
    .string()
    .trim()
    .regex(/^\d+$/, "A port number.")
    .refine((value) => Number(value) >= 1 && Number(value) <= 65535, "A port between 1 and 65535."),
  ttl: z
    .string()
    .trim()
    .regex(/^\d+$/, "A number of minutes.")
    .refine((value) => Number(value) >= 1 && Number(value) <= 1440, "1 to 1440 minutes."),
});
export type PreviewValues = z.infer<typeof previewSchema>;

export const incidentSchema = z.object({
  title: z.string().trim().min(1, "A title is required.").max(200, "At most 200 characters."),
  severity: z.enum(["sev1", "sev2", "sev3", "sev4"]),
});
export type IncidentValues = z.infer<typeof incidentSchema>;

export const scimSchema = z.object({
  name: z.string().trim().min(1, "A name is required.").max(80, "At most 80 characters."),
});
export type ScimValues = z.infer<typeof scimSchema>;

export const githubAppSchema = z.object({
  installation: z
    .string()
    .trim()
    .regex(/^\d+$/, "The installation id is a number.")
    .refine((value) => Number(value) > 0, "A positive installation id."),
});
export type GitHubAppValues = z.infer<typeof githubAppSchema>;
