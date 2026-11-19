import { describe, expect, it } from "vitest";

import { acceptInviteSchema, aiProviderSchema, apiKeySchema, approvalsSchema, baselineSchema, checksSchema, commandSchema, componentEntrySchema, deleteProjectSchema, designTokenSchema, environmentSchema, inviteSchema, loginSchema, mcpSchema, modelsSchema, pluginChecksSchema, policySchema, registerSchema, repoConnectSchema, repoCredentialSchema, ruleSchema, scaffoldSchema, taskComposerSchema, toolsSchema, windowSchema, type TaskComposerValues } from "./schemas";

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

describe("access schemas", () => {
  it("requires a valid invite email", () => {
    expect(inviteSchema.safeParse({ email: "dev@acme.io", role: "member" }).success).toBe(true);
    expect(inviteSchema.safeParse({ email: "nope", role: "member" }).success).toBe(false);
  });

  it("requires a key name and a bounded expiry", () => {
    expect(apiKeySchema.safeParse({ name: "ci", role: "member", days: "90" }).success).toBe(true);
    expect(apiKeySchema.safeParse({ name: "", role: "member", days: "90" }).success).toBe(false);
    expect(apiKeySchema.safeParse({ name: "ci", role: "member", days: "-1" }).success).toBe(false);
    expect(apiKeySchema.safeParse({ name: "ci", role: "member", days: "4000" }).success).toBe(false);
  });

  it("requires a rule name and body", () => {
    expect(ruleSchema.safeParse({ name: "Tests", level: "organization", body: "Every change has a test." }).success).toBe(true);
    expect(ruleSchema.safeParse({ name: "", level: "organization", body: "x" }).success).toBe(false);
    expect(ruleSchema.safeParse({ name: "Tests", level: "organization", body: "" }).success).toBe(false);
  });
});

describe("project settings schemas", () => {
  it("caps the required checks string", () => {
    expect(checksSchema.safeParse({ checks: "ci, test" }).success).toBe(true);
    expect(checksSchema.safeParse({ checks: "x".repeat(501) }).success).toBe(false);
  });

  it("requires a baseline name", () => {
    expect(baselineSchema.safeParse({ name: "home" }).success).toBe(true);
    expect(baselineSchema.safeParse({ name: "  " }).success).toBe(false);
  });

  it("requires the project name confirmation", () => {
    expect(deleteProjectSchema.safeParse({ confirm: "Portal" }).success).toBe(true);
    expect(deleteProjectSchema.safeParse({ confirm: "" }).success).toBe(false);
  });
});

describe("project settings policy schemas", () => {
  it("requires a scaffold component name", () => {
    expect(scaffoldSchema.safeParse({ name: "ButtonGroup", kind: "react" }).success).toBe(true);
    expect(scaffoldSchema.safeParse({ name: "  ", kind: "react" }).success).toBe(false);
  });

  it("validates the MCP JSON", () => {
    expect(mcpSchema.safeParse({ config: "" }).success).toBe(true);
    expect(mcpSchema.safeParse({ config: "[]" }).success).toBe(true);
    expect(mcpSchema.safeParse({ config: "[oops" }).success).toBe(false);
  });

  it("requires a numeric max diff", () => {
    expect(policySchema.safeParse({ max: "0", paths: "" }).success).toBe(true);
    expect(policySchema.safeParse({ max: "abc", paths: "" }).success).toBe(false);
  });

  it("caps the plugin checks at five", () => {
    expect(pluginChecksSchema.safeParse({ checks: "[]" }).success).toBe(true);
    const six = JSON.stringify(Array.from({ length: 6 }, (_, i) => ({ name: `c${i}`, command: "true" })));
    expect(pluginChecksSchema.safeParse({ checks: six }).success).toBe(false);
  });

  it("validates the window JSON and the approvals", () => {
    expect(windowSchema.safeParse({ window: "" }).success).toBe(true);
    expect(windowSchema.safeParse({ window: "{bad" }).success).toBe(false);
    expect(modelsSchema.safeParse({ models: "gpt-5" }).success).toBe(true);
    expect(toolsSchema.safeParse({ tools: "mcp__fs__read" }).success).toBe(true);
    expect(approvalsSchema.safeParse({ approvals: "2" }).success).toBe(true);
    expect(approvalsSchema.safeParse({ approvals: "4" }).success).toBe(false);
  });
});
