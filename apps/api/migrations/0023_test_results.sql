-- +goose Up
CREATE TYPE check_status AS ENUM ('pending','running','passed','failed');

CREATE TABLE test_results (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    repo_id         uuid REFERENCES repos(id) ON DELETE CASCADE,
    pull_request_id uuid REFERENCES pull_requests(id) ON DELETE CASCADE,
    run_id          uuid REFERENCES runs(id) ON DELETE SET NULL,
    name            text NOT NULL,
    status          check_status NOT NULL DEFAULT 'pending',
    output          text NOT NULL DEFAULT '',
    duration_ms     int NOT NULL DEFAULT 0,
    commit_sha      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (pull_request_id, name)
);

ALTER TABLE test_results ENABLE ROW LEVEL SECURITY;
CREATE POLICY test_results_tenant ON test_results USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE test_results;
DROP TYPE check_status;
