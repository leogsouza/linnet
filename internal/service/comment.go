package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Comment struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"-"`
	PostID     int64     `json:""`
	Content    string    `json:"content"`
	LikesCount int       `json:"likes_count"`
	CreatedAt  time.Time `json:"created_at"`
	User       *User     `json:"user,omitempty"`
	Mine       bool      `json:"mine"`
	Liked      bool      `json:"liked"`
}

func (s *Service) CreateComment(ctx context.Context, postID int64, content string) (Comment, error) {
	var c Comment
	uid, ok := ctx.Value(KeyAuthUserID).(int64)
	if !ok {
		return c, ErrUnauthenticated
	}

	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 480 {
		return c, ErrInvalidContent
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return c, fmt.Errorf("could not begin tx: %v", err)
	}

	defer tx.Rollback()

	query := `
		INSERT INTO comments(user_id, post_id, content) VALUES($1, $2, $3)
		RETURNING id, created_at`
	err = tx.QueryRowContext(ctx, query, uid, postID, content).Scan(&c.ID, &c.CreatedAt)
	if isForeignKeyViolation(err) {
		return c, ErrPostNotFound
	}

	if err != nil {
		return c, fmt.Errorf("could not insert comment: %v", err)
	}

	c.UserID = uid
	c.PostID = postID
	c.Content = content
	c.Mine = true

	query = "UPDATE posts SET comments_count = comments_count + 1 WHERE id = $1"
	if _, err = tx.ExecContext(ctx, query, postID); err != nil {
		return c, fmt.Errorf("could not update and increment post comments count: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return c, fmt.Errorf("could not commit to create comment: %v", err)
	}

	return c, nil
}
