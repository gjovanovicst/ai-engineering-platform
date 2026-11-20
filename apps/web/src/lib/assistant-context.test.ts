import { describe, expect, it } from "vitest";

import { pageSelection } from "./assistant-context";

describe("pageSelection", () => {
  it("reads the project, the run and the PR from the route", () => {
    expect(pageSelection("/projects/982a-1/runs/abc-2")).toEqual({
      project_id: "982a-1",
      run_id: "abc-2",
    });
    expect(pageSelection("/projects/982a-1/pulls/7")).toEqual({ project_id: "982a-1", pr_id: "7" });
    expect(pageSelection("/projects/982a-1/tasks/t-9")).toEqual({ project_id: "982a-1", task_id: "t-9" });
  });

  it("is empty elsewhere", () => {
    expect(pageSelection("/overview")).toEqual({});
    expect(pageSelection("/settings")).toEqual({});
  });
});
