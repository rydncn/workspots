-- One vote per member per spot, and one definition of what the public sees.
--
-- A member could post many reports on one spot and outvote everyone else, so
-- spot_attributes now counts only each member's latest visible report per
-- spot; earlier reports stay on the page as history. Reports always have an
-- author (migration 0003 deletes them with the account), so the column is
-- made NOT NULL; rows without one can only be hand-inserted development data.

DELETE FROM reports WHERE author_id IS NULL;
ALTER TABLE reports ALTER COLUMN author_id SET NOT NULL;

-- What anyone may see: approved spots, and unhidden reports on them. Every
-- query about public visibility reads these views, so the rule lives here.
CREATE VIEW public_spots WITH (security_invoker = true) AS
  SELECT * FROM spots WHERE status = 'approved';
CREATE VIEW public_reports WITH (security_invoker = true) AS
  SELECT r.* FROM reports r JOIN public_spots s ON s.id = r.spot_id WHERE NOT r.hidden;

DROP VIEW spot_attributes;
CREATE VIEW spot_attributes WITH (security_invoker = true) AS
WITH votes AS (
  -- each member's latest visible report on each spot
  SELECT DISTINCT ON (spot_id, author_id) *
  FROM reports WHERE NOT hidden
  ORDER BY spot_id, author_id, created_at DESC, id DESC
)
SELECT s.id AS spot_id,
       (SELECT count(*) FROM reports r WHERE r.spot_id = s.id AND NOT r.hidden)::int AS report_count,
       coalesce(round(percentile_cont(0.5) WITHIN GROUP (ORDER BY v.wifi)),  s.wifi)::smallint  AS wifi,
       coalesce(round(percentile_cont(0.5) WITHIN GROUP (ORDER BY v.noise)), s.noise)::smallint AS noise,
       coalesce(avg(v.power::int) > 0.5,             s.power)             AS power,
       coalesce(avg(v.time_limit::int) > 0.5,        s.time_limit)        AS time_limit,
       coalesce(avg(v.step_free::int) > 0.5,         s.step_free)         AS step_free,
       coalesce(avg(v.accessible_toilet::int) > 0.5, s.accessible_toilet) AS accessible_toilet,
       coalesce(avg(v.quiet_space::int) > 0.5,       s.quiet_space)       AS quiet_space
FROM spots s
LEFT JOIN votes v ON v.spot_id = s.id
GROUP BY s.id;

REVOKE INSERT, UPDATE, DELETE ON public_spots, public_reports, spot_attributes FROM workspots_app;
