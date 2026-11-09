-- Accounts and sessions (slice 3), and the author links the earlier tables
-- left open. Deleting a user deletes their sessions and reports (ADR 0011);
-- a spot they added keeps existing with no author.

CREATE TABLE users (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email         text NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
  password_hash text NOT NULL,
  role          text NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'moderator', 'admin')),
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_idx ON users (lower(email));

-- id is the SHA-256 of the token the browser holds, so a database read never
-- yields a usable cookie.
CREATE TABLE sessions (
  id         text PRIMARY KEY,
  user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

ALTER TABLE reports ADD CONSTRAINT reports_author_fk FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE;
ALTER TABLE spots   ADD CONSTRAINT spots_created_by_fk FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL;
