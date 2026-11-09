-- Declared attributes on a spot, reports with one rating per attribute, and a
-- view giving each spot's effective attributes: the median (or majority) of
-- its visible reports, or the declared value when it has none.
--
-- Scales: wifi 0 none, 1 poor, 2 usable, 3 good, 4 excellent.
--         noise 1 quiet, 2 moderate, 3 loud.
--         time_limit true means seating is time-limited.

ALTER TABLE spots
  ADD COLUMN wifi              smallint NOT NULL DEFAULT 2 CHECK (wifi BETWEEN 0 AND 4),
  ADD COLUMN noise             smallint NOT NULL DEFAULT 2 CHECK (noise BETWEEN 1 AND 3),
  ADD COLUMN power             boolean  NOT NULL DEFAULT false,
  ADD COLUMN time_limit        boolean  NOT NULL DEFAULT false,
  ADD COLUMN step_free         boolean  NOT NULL DEFAULT false,
  ADD COLUMN accessible_toilet boolean  NOT NULL DEFAULT false,
  ADD COLUMN quiet_space       boolean  NOT NULL DEFAULT false;

CREATE TABLE reports (
  id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  spot_id           bigint NOT NULL REFERENCES spots (id) ON DELETE CASCADE,
  author_id         bigint,  -- references users(id) from migration 0003; null once the author is deleted
  wifi              smallint NOT NULL CHECK (wifi BETWEEN 0 AND 4),
  noise             smallint NOT NULL CHECK (noise BETWEEN 1 AND 3),
  power             boolean  NOT NULL,
  time_limit        boolean  NOT NULL,
  step_free         boolean  NOT NULL,
  accessible_toilet boolean  NOT NULL,
  quiet_space       boolean  NOT NULL,
  note              text     NOT NULL DEFAULT '' CHECK (length(note) <= 500),
  hidden            boolean  NOT NULL DEFAULT false,
  created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reports_spot_idx ON reports (spot_id, hidden, created_at DESC);

-- Ties are decided on purpose: the median of an even count is the rounded
-- midpoint (ratings 1 and 3 give 2), and a boolean needs a strict majority
-- (a 1-1 tie is "no", the cautious answer for access attributes). The view
-- runs with the caller's privileges and is read-only for the app role.
CREATE VIEW spot_attributes WITH (security_invoker = true) AS
SELECT s.id AS spot_id,
       count(r.id)::int AS report_count,
       coalesce(round(percentile_cont(0.5) WITHIN GROUP (ORDER BY r.wifi)),  s.wifi)::smallint  AS wifi,
       coalesce(round(percentile_cont(0.5) WITHIN GROUP (ORDER BY r.noise)), s.noise)::smallint AS noise,
       coalesce(avg(r.power::int) > 0.5,             s.power)             AS power,
       coalesce(avg(r.time_limit::int) > 0.5,        s.time_limit)        AS time_limit,
       coalesce(avg(r.step_free::int) > 0.5,         s.step_free)         AS step_free,
       coalesce(avg(r.accessible_toilet::int) > 0.5, s.accessible_toilet) AS accessible_toilet,
       coalesce(avg(r.quiet_space::int) > 0.5,       s.quiet_space)       AS quiet_space
FROM spots s
LEFT JOIN reports r ON r.spot_id = s.id AND NOT r.hidden
GROUP BY s.id;
REVOKE INSERT, UPDATE, DELETE ON spot_attributes FROM workspots_app;
