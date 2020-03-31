package service

import (
	"context"
	"database/sql"
	"fmt"
)

// TimelineItem represents an item on user's timeline
type TimelineItem struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"-"`
	PostID int64 `json:"-"`
	Post   Post  `json:"post"`
}

// Timeline retrieves the user's timeline
func (s *Service) Timeline(
	ctx context.Context,
	last int,
	before int64,
) ([]TimelineItem, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(int64)
	if !ok {
		return nil, ErrUnauthenticated
	}

	last = normalizePage(last)
	query, args, err := buildQuery(`
		SELECT timeline.id,posts.id, content, spoiler_of, nsfw, likes_count, created_at
		, posts.user_id = @uid AS mine
		, likes.user_id IS NOT NULL AS liked
		, users.username, users.avatar
		FROM timeline
		INNER JOIN posts ON timeline.post_id = posts.id
		INNER JOIN users ON posts.user_id = users.id
		LEFT JOIN post_likes as likes
		ON likes.user_id = @uid AND likes.post_id = posts.id
		WHERE timeline.user_id = @uid
		{{if .before}}AND timeline.id < @before{{end}}
		ORDER BY created_at DESC
		LIMIT @last
	`, map[string]interface{}{
		"uid":    uid,
		"last":   last,
		"before": before,
	})

	if err != nil {
		return nil, fmt.Errorf("could not build timeline sql query; %v", err)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select timeline: %v", err)
	}

	defer rows.Close()

	tt := make([]TimelineItem, 0, last)
	for rows.Next() {
		var ti TimelineItem
		var u User
		var avatar sql.NullString
		dest := []interface{}{
			&ti.ID,
			&ti.Post.ID,
			&ti.Post.Content,
			&ti.Post.SpoilerOf,
			&ti.Post.NSFW,
			&ti.Post.LikesCount,
			&ti.Post.CreatedAt,
			&ti.Post.Mine,
			&ti.Post.Liked,
			&u.Username,
			&avatar,
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan timeline item: %v", err)
		}

		if avatar.Valid {
			avatarURL := s.origin + "/img/avatars/" + avatar.String
			u.AvatarURL = &avatarURL
		}

		ti.Post.User = &u

		tt = append(tt, ti)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate timeline rows: %v", err)
	}

	return tt, nil
}
