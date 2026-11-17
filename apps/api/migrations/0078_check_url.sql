-- 0078_check_url: remember where a CI check ran.
--
-- A check's output is often empty (GitHub Actions puts the failure in the job
-- log, not in the check-run summary). The URL lets the platform link a failed
-- check to the job page so a human or an agent can open the log instead of
-- guessing.
ALTER TABLE test_results ADD COLUMN url text NOT NULL DEFAULT '';
