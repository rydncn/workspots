-- Who decided a spot and when (slice 5, outside review): set by the same
-- update that moves it out of pending, shown in the queue's history. A
-- deleted moderator leaves the decision with no name (ADR 0011).

ALTER TABLE spots
  ADD COLUMN moderated_by bigint REFERENCES users (id) ON DELETE SET NULL,
  ADD COLUMN moderated_at timestamptz;
CREATE INDEX spots_moderated_at_idx ON spots (moderated_at DESC) WHERE moderated_at IS NOT NULL;
