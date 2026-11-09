-- Moderation (slice 5): why and when a report was hidden, so the queue can
-- show it (AC20). A report hidden before this migration has no reason.

ALTER TABLE reports
  ADD COLUMN hidden_reason text NOT NULL DEFAULT '' CHECK (length(hidden_reason) <= 500),
  ADD COLUMN hidden_at     timestamptz;
