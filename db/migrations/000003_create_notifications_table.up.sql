CREATE TABLE IF NOT EXISTS notifications (
  id SERIAL NOT NULL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users,
  actors VARCHAR[] NOT NULL,
  type VARCHAR NOT NULL,
  read BOOLEAN NOT NULL DEFAULT false,
  issued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sorted_notifications ON notifications (issued_at DESC);
