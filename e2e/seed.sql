-- Known spots for the end-to-end tests, including worst-case data: a long
-- name, an unbreakable word, a spot with no reports, and a hidden report.
INSERT INTO spots (name, kind, area, address, opening_hours, status, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space) VALUES
 ('Riverside Café', 'cafe', 'Leeds, LS1', '12 Calls Landing, Leeds LS1 7BR', E'Mon-Fri 7:30-18:00\nSat-Sun 9:00-17:00', 'approved', 3, 2, true, false, true, true, false),
 ('Headingley Library', 'library', 'Leeds, LS6', 'North Lane, Leeds LS6 3HG', 'Mon-Sat 9:00-17:00', 'approved', 4, 1, true, false, true, true, true),
 ('The Extraordinarily Long Named Coworking Space Of Supercalifragilisticexpialidocious Proportions', 'coworking', 'York', '1 Micklegate, York YO1 6JH', '', 'approved', 4, 2, true, true, false, false, true),
 ('Pending Place', 'cafe', 'Leeds', '1 Nowhere St', '', 'pending', 2, 2, false, false, false, false, false),
 -- For the report journeys, so their reports don't move the attributes the browse journeys assert on.
 ('Reportable Café', 'cafe', 'Bradford', '9 Market St, Bradford BD1 1AA', 'Daily 8-20', 'approved', 2, 2, true, false, true, false, false),
 -- For the moderation journeys: one flagged report per project is hidden here.
 ('Moderated Café', 'cafe', 'Wakefield', '2 Cross St, Wakefield WF1 1AB', 'Daily 9-17', 'approved', 2, 2, true, false, true, false, false);

-- Members for the end-to-end tests to sign in as (password "a long enough
-- password", hashed once with the app's own argon2id parameters), so that
-- only the sign-up journey signs up and the in-memory rate limits (ten
-- sign-ups per IP per quarter hour) stay out of the way of the suite.
INSERT INTO users (email, password_hash) VALUES
 ('member-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko'),
 ('member-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko'),
 ('reader-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko'),
 ('reader-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko');
-- Moderators, admins and the members the admins promote (slice 5), one per project.
INSERT INTO users (email, password_hash, role) VALUES
 ('moderator-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'moderator'),
 ('moderator-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'moderator'),
 ('admin-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'admin'),
 ('admin-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'admin'),
 ('promoted-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'member'),
 ('promoted-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko', 'member');
INSERT INTO users (email, password_hash) VALUES
 ('contributor-desktop@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko'),
 ('contributor-narrow-320@example.test', '$argon2id$v=19$m=19456,t=2,p=1$3HDz/F3fYJvs0Ex/JDZ4gw$Avo7ijU1q/OtFAqZDfN4tuMxYSKrGFBZ6rjpUoky5Ko');

INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note, hidden, created_at)
SELECT id, (SELECT id FROM users WHERE email = 'member-desktop@example.test'), 4, 1, true, false, true, true, false, 'Big tables upstairs, sockets along the wall.', false, now() - interval '3 days' FROM spots WHERE name = 'Riverside Café';
INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note, hidden, created_at)
SELECT id, (SELECT id FROM users WHERE email = 'reader-desktop@example.test'), 3, 2, true, false, true, true, false, 'Busy at lunch.', false, now() - interval '1 day' FROM spots WHERE name = 'Riverside Café';
INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note, hidden, created_at)
SELECT id, (SELECT id FROM users WHERE email = 'reader-narrow-320@example.test'), 0, 3, false, true, false, false, false, 'HIDDEN SPAM NOTE', true, now() FROM spots WHERE name = 'Riverside Café';


-- One flagged report per project on Moderated Café, for the hide journey.
INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note, created_at)
SELECT id, (SELECT id FROM users WHERE email = 'reader-desktop@example.test'), 4, 1, true, false, true, false, false, 'FLAGGED NOTE desktop', now() - interval '2 days' FROM spots WHERE name = 'Moderated Café';
INSERT INTO reports (spot_id, author_id, wifi, noise, power, time_limit, step_free, accessible_toilet, quiet_space, note, created_at)
SELECT id, (SELECT id FROM users WHERE email = 'reader-narrow-320@example.test'), 4, 1, true, false, true, false, false, 'FLAGGED NOTE narrow-320', now() - interval '1 day' FROM spots WHERE name = 'Moderated Café';
INSERT INTO problem_reports (report_id, reporter_id, reason)
SELECT r.id, (SELECT id FROM users WHERE email = 'member-desktop@example.test'), 'Advertising, not a report (' || replace(r.note, 'FLAGGED NOTE ', '') || ').'
FROM reports r WHERE r.note LIKE 'FLAGGED NOTE %';
