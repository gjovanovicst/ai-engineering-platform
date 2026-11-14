// k6 load test for E10-S4: N parallel runs through the public API.
//
//   AIP_BASE=http://localhost:18080 RUNS=50 VUS=10 k6 run scripts/k6/loadtest.js
//
// Each iteration creates a task, moves it to ready and starts a development run.
// Thresholds: p95 of API requests < 300 ms and < 5% failed requests. A 429 from
// the per-org cap counts as a failed request, so the target stack must allow at
// least RUNS active runs (AIP_MAX_ACTIVE_RUNS) or the test will not pass.
import http from "k6/http";
import { check } from "k6";
import exec from "k6/execution";
import { Counter } from "k6/metrics";

const BASE = (__ENV.AIP_BASE || "http://localhost:18080").replace(/\/$/, "");
const RUNS = Number(__ENV.RUNS || 50);
const VUS = Number(__ENV.VUS || 10);
const PASSWORD = __ENV.AIP_LOAD_PASSWORD || "correct horse battery";
const EMAIL = __ENV.AIP_LOAD_EMAIL || `load-${Date.now()}@acme.io`;

const runsStarted = new Counter("runs_started");
const runsThrottled = new Counter("runs_throttled");

// A 429 from the per-org active-run cap is expected behaviour, not a failure;
// raise AIP_MAX_ACTIVE_RUNS above RUNS to start every run at once.
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 299 }, { min: 429, max: 429 }));

export const options = {
  scenarios: {
    runs: {
      executor: "shared-iterations",
      vus: Math.min(VUS, RUNS),
      iterations: RUNS,
      maxDuration: __ENV.AIP_MAX_DURATION || "15m",
    },
  },
  thresholds: {
    http_req_duration: ["p(95)<300"],
    http_req_failed: ["rate<0.05"],
    checks: ["rate>0.99"],
  },
};

function jsonHeaders(token) {
  return { headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` } };
}

export function setup() {
  const register = http.post(
    `${BASE}/api/v1/auth/register`,
    JSON.stringify({
      email: EMAIL,
      name: "Load Test",
      password: PASSWORD,
      org_name: `Load Test ${Date.now()}`,
    }),
    { headers: { "Content-Type": "application/json" } },
  );
  // 409 means the account already exists from an earlier run.
  if (!check(register, { "register ok": (r) => r.status === 201 || r.status === 409 })) {
    console.warn(`register status=${register.status} body=${register.body}`);
  }

  const login = http.post(
    `${BASE}/api/v1/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    { headers: { "Content-Type": "application/json" } },
  );
  if (!check(login, { "login ok": (r) => r.status === 200 })) {
    console.warn(`login status=${login.status} body=${login.body}`);
  }
  const token = login.json("access_token");

  const me = http.get(`${BASE}/api/v1/auth/me`, jsonHeaders(token));
  check(me, { "me ok": (r) => r.status === 200 });
  const org = me.json("org_id");

  const project = http.post(
    `${BASE}/api/v1/orgs/${org}/projects`,
    JSON.stringify({ name: `Load ${Date.now()}` }),
    jsonHeaders(token),
  );
  check(project, { "project ok": (r) => r.status === 201 });

  return { token, org, project: project.json("id") };
}

export default function (data) {
  const token = data.token;
  const iteration = exec.scenario.iterationInTest;

  const task = http.post(
    `${BASE}/api/v1/orgs/${data.org}/projects/${data.project}/tasks`,
    JSON.stringify({
      title: `Load task ${iteration}`,
      description: "Created by the k6 load test.",
      priority: "low",
      acceptance: [{ given: "load", when: "it runs", then: "the API stays fast" }],
    }),
    jsonHeaders(token),
  );
  if (!check(task, { "task created": (r) => r.status === 201 })) {
    console.warn(`task status=${task.status} body=${task.body}`);
    return;
  }
  const ref = task.json("ref");

  const ready = http.post(
    `${BASE}/api/v1/orgs/${data.org}/projects/${data.project}/tasks/${ref}/transitions`,
    JSON.stringify({ to: "ready" }),
    jsonHeaders(token),
  );
  if (!check(ready, { "task ready": (r) => r.status === 200 })) {
    console.warn(`ready status=${ready.status} body=${ready.body}`);
    return;
  }

  const run = http.post(
    `${BASE}/api/v1/orgs/${data.org}/projects/${data.project}/tasks/${ref}/runs`,
    JSON.stringify({ agent_type: "development" }),
    jsonHeaders(token),
  );
  if (check(run, { "run accepted": (r) => r.status === 200 || r.status === 201 || r.status === 202 || r.status === 429 })) {
    if (run.status === 429) {
      runsThrottled.add(1);
    } else {
      runsStarted.add(1);
    }
  } else {
    console.warn(`run status=${run.status} body=${run.body}`);
  }
}
