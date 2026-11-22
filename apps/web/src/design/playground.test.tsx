import { describe, expect, it } from "vitest";

import contracts from "./contracts.json";
import { playgroundExamples } from "./playground";

describe("the component playground", () => {
  it("has an example for every registered component", () => {
    for (const component of contracts.components) {
      expect(playgroundExamples[component.name], `${component.name} needs an example`).toBeTypeOf("function");
    }
  });

  it("registers no example for a component outside the contract", () => {
    const names = new Set(contracts.components.map((component) => component.name));
    for (const name of Object.keys(playgroundExamples)) {
      expect(names.has(name), `${name} is not in design/contracts.json`).toBe(true);
    }
  });
});
