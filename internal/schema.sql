DROP DATABASE IF EXISTS linnet CASCADE;
CREATE DATABASE IF NOT EXISTS linnet;
SET DATABASE = linnet;


CREATE TABLE IF NOT EXISTS users (
  id SERIAL NOT NULL PRIMARY KEY,
  email VARCHAR NOT NULL UNIQUE,
  username VARCHAR NOT NULL UNIQUE,
  followers_count INT NOT NULL DEFAULT 0 CHECK (followers_count >= 0),
  followees_count INT NOT NULL DEFAULT 0 CHECK (followees_count >= 0)
);

CREATE TABLE IF NOT EXISTS follows (
  follower_id INT NOT NULL REFERENCES users,
  followee_id INT NOT NULL REFERENCES users,
  PRIMARY KEY (follower_id, followee_id)
);

INSERT INTO users (id, email, username) VALUES
  (1, 'test@test.com', 'Test'),
  (2, 'john@doe.com', 'John');