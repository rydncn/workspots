-- Problem reports (slice 4): a member flags a spot or a report for the
-- moderation queue. Exactly one target. Deleted with the reporter (ADR 0011).

CREATE TABLE problem_reports (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  spot_id     bigint REFERENCES spots (id) ON DELETE CASCADE,
  report_id   bigint REFERENCES reports (id) ON DELETE CASCADE,
  reporter_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  reason      text NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
  status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  CHECK ((spot_id IS NULL) <> (report_id IS NULL))
);
CREATE INDEX problem_reports_open_idx ON problem_reports (status, created_at);

-- A member's own spots (the account page) and the author cascade on deletion.
CREATE INDEX spots_created_by_idx ON spots (created_by);
