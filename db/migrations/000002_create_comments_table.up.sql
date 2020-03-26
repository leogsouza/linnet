CREATE TABLE IF NOT EXISTS comments (
  id SERIAL NOT NULL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users,
  post_id INT NOT NULL REFERENCES posts,
  content VARCHAR NOT NULL,
  likes_count INT NOT NULL DEFAULT 0 CHECK (likes_count >= 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sorted_comments on comments (created_at DESC);

CREATE TABLE IF NOT EXISTS comment_likes (
  user_id INT NOT NULL REFERENCES users,
  comment_id INT NOT NULL REFERENCES comments,
  PRIMARY KEY(user_id, comment_id)
);

INSERT INTO comments (id, user_id, post_id, content) VALUES
  (1, 1, 1, 'sample comment');

ALTER TABLE IF EXISTS posts ADD COLUMN IF NOT EXISTS comments_count INT NOT NULL DEFAULT 0 CHECK (comments_count >= 0);