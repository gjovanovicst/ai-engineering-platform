import { describe, expect, it } from "vitest";

import { acceptInviteSchema, aiProviderSchema, commandSchema, componentEntrySchema, designTokenSchema, environmentSchema, loginSchema, registerSchema, repoConnectSchema, repoCredentialSchema, taskComposerSchema, type TaskComposerValues } from "./schemas";

function composer(overrides: Partial<TaskComposerValues> = {}): TaskComposerValues {
  return {
    title: "Add a health endpoint",
    description: "",
    references: "",
    priority: "med",
    tags: "",
    acceptance: [{ given: "the API runs", when: "GET /health", then: "200 ok" }],
    files: "",
    outOfScope: "",
    testPlan: "",
    rollback: "",
    dependsOn: [],
    ...overrides,
  };
}

describe("task composer schema", () => {
  it("rejects an empty title", () => {
    const result = taskComposerSchema.safeParse(composer({ title: "   " }));
    expect(result.success).toBe(false);
  });

  it("rejects a title longer than 200 characters", () => {
    expect(taskComposerSchema.safeParse(composer({ title: "x".repeat(201) })).success).toBe(false);
  });

  it("accepts a minimal payload", () => {
    expect(taskComposerSchema.safeParse(composer()).success).toBe(true);
  });
});

describe("command schema", () => {
  it("accepts a one-line command", () => {
    expect(commandSchema.safeParse({ command: "go test ./..." }).success).toBe(true);
  });

  it("rejects a multi-line command", () => {
    expect(commandSchema.safeParse({ command: "go test ./...\nrm -rf /" }).success).toBe(false);
  });

  it("rejects a command longer than 500 characters", () => {
    expect(commandSchema.safeParse({ command: "x".repeat(501) }).success).toBe(false);
  });
});

describe("registry schemas", () => {
  it("requires a component name and caps the purpose", () => {
    const base = {
      name: "Button", status: "approved", category: "form", purpose: "", rules: "",
      framework: "", package: "", import_path: "", version: "", docs: "", example: "",
    };
    expect(componentEntrySchema.safeParse(base).success).toBe(true);
    expect(componentEntrySchema.safeParse({ ...base, name: "  " }).success).toBe(false);
    expect(componentEntrySchema.safeParse({ ...base, purpose: "x".repeat(501) }).success).toBe(false);
  });

  it("requires a token name and value", () => {
    expect(designTokenSchema.safeParse({ name: "color-brand", value: "#2563eb", category: "color" }).success).toBe(true);
    expect(designTokenSchema.safeParse({ name: "", value: "#2563eb", category: "color" }).success).toBe(false);
    expect(designTokenSchema.safeParse({ name: "a", value: "", category: "color" }).success).toBe(false);
  });
});

describe("auth schemas", () => {
  it("requires a valid email and a password", () => {
    expect(loginSchema.safeParse({ email: "not-an-email", password: "x" }).success).toBe(false);
    expect(loginSchema.safeParse({ email: "a@b.co", password: "" }).success).toBe(false);
    expect(loginSchema.safeParse({ email: "a@b.co", password: "secret" }).success).toBe(true);
  });

  it("rejects a password shorter than 12 characters on register", () => {
    const base = { name: "Jane", email: "a@b.co", org_name: "", password: "short" };
    expect(registerSchema.safeParse(base).success).toBe(false);
    expect(registerSchema.safeParse({ ...base, password: "correct horse battery" }).success).toBe(true);
  });

  it("accepts an invite with a valid password", () => {
    expect(acceptInviteSchema.safeParse({ name: "Jane", password: "correct horse battery" }).success).toBe(true);
    expect(acceptInviteSchema.safeParse({ name: "", password: "correct horse battery" }).success).toBe(false);
  });
});

describe("delivery panel schemas", () => {
  it("requires a repository name", () => {
    const base = { provider: "github", name: "acme/app", default_branch: "main", webhook_secret: "s" };
    expect(repoConnectSchema.safeParse(base).success).toBe(true);
    expect(repoConnectSchema.safeParse({ ...base, name: "  " }).success).toBe(false);
    expect(repoConnectSchema.safeParse({ ...base, provider: "gitea" }).success).toBe(false);
  });

  it("requires a provider name and base URL", () => {
    const base = { name: "OpenAI", base_url: "https://api.openai.com/v1", model: "", api_key: "sk" };
    expect(aiProviderSchema.safeParse(base).success).toBe(true);
    expect(aiProviderSchema.safeParse({ ...base, name: "" }).success).toBe(false);
    expect(aiProviderSchema.safeParse({ ...base, base_url: "" }).success).toBe(false);
  });

  it("requires a token", () => {
    expect(repoCredentialSchema.safeParse({ token: "github_pat_x" }).success).toBe(true);
    expect(repoCredentialSchema.safeParse({ token: "" }).success).toBe(false);
  });

  it("requires an environment URL", () => {
    expect(environmentSchema.safeParse({ name: "dev", url: "https://dev.example.com" }).success).toBe(true);
    expect(environmentSchema.safeParse({ name: "dev", url: "" }).success).toBe(false);
  });
});
