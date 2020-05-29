CREATE TABLE IF NOT EXISTS post_subscriptions (
  user_id INT NOT NULL REFERENCES users,
  post_id INT NOT NULL REFERENCES posts,
  PRIMARY KEY(user_id, post_id)
);

-- ALTER TABLE IF EXISTS notifications ADD COLUMN IF NOT EXISTS post_id INT REFERENCES posts;
-- Use this way because cockroach doesn't support add column with references
-- https://github.com/cockroachdb/cockroach/issues/32917
ALTER TABLE IF EXISTS notifications ADD COLUMN IF NOT EXISTS post_id INT;
CREATE INDEX IF NOT EXISTS post_idx on notifications (post_id);
ALTER TABLE IF EXISTS notifications  ADD CONSTRAINT post_fk FOREIGN KEY (post_id) REFERENCES posts (id);

CREATE UNIQUE INDEX IF NOT EXISTS unique_notifications on notifications (user_id, type, post_id, read);