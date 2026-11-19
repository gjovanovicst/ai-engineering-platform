/** Plain-language definitions of the platform's terms (the help tips). */
export const glossary: Record<string, { term: string; definition: string }> = {
  run: {
    term: "Run",
    definition:
      "One agent execution for a task: a planning, development, review or fix run. It starts with the task context, uses the sandbox and ends when the gates pass or it fails.",
  },
  plan: {
    term: "Plan",
    definition:
      "The steps the agent intends to take before it writes code. A planning run parks in awaiting approval until a human approves or rejects the plan.",
  },
  acceptance: {
    term: "Acceptance criteria",
    definition:
      "Given/when/then statements that define what done means. At least one complete criterion is required before a task is ready.",
  },
  dor: {
    term: "Definition of Ready",
    definition:
      "The minimum a task needs before an agent can start: a title and at least one complete acceptance criterion; the scope fields are recommended.",
  },
  gate: {
    term: "Gate",
    definition:
      "An automatic check a run must pass before its PR is ready: the test suite, the secret scan, the acceptance proof and the agent review.",
  },
  check: {
    term: "Check",
    definition:
      "One reported status on a pull request (ci, test, visual, review). The required checks must pass before a human can merge.",
  },
  pr: {
    term: "Pull request",
    definition:
      "The branch the agent pushed and the diff it proposes. The platform opens it; a human reviews and merges it.",
  },
  review: {
    term: "Agent review",
    definition:
      "A review run that reads the diff and reports findings. Blockers must be fixed before the PR is ready; minor findings are suggestions.",
  },
  ttl: {
    term: "TTL",
    definition: "Time to live: how long a preview environment exists before it is destroyed.",
  },
  mcp: {
    term: "MCP",
    definition:
      "Model Context Protocol: the servers a run may call as tools (a file system, git, a search). The project policy can allow only some tools.",
  },
  byok: {
    term: "BYOK",
    definition:
      "Bring your own key: the organisation can use its own KMS to encrypt the provider and repository secrets.",
  },
  scim: {
    term: "SCIM",
    definition: "The provisioning protocol an identity provider uses to create and remove members automatically.",
  },
  rpo: {
    term: "RPO",
    definition: "Recovery point objective: how much data may be lost, measured as the age of the last successful backup.",
  },
  rto: {
    term: "RTO",
    definition: "Recovery time objective: how long a restore may take before the service is considered down too long.",
  },
};
