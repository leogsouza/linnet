package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sanity-io/litter"
)

var (
	// ErrInvalidContent used when the content is invalid.
	ErrInvalidContent = errors.New("invalid content")
	//ErrInvalidSpoiler used for invalid spoiler title.
	ErrInvalidSpoiler = errors.New("invalid spoiler")
	// ErrPostNotFound used for post not found.
	ErrPostNotFound = errors.New("post not found")
)

// Post Model
type Post struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"-"`
	Content    string    `json:"content"`
	SpoilerOf  *string   `json:"spoiler_of"`
	NSFW       bool      `json:"nsfw"`
	LikesCount int       `json:"likesCount"`
	CreatedAt  time.Time `json:"created_at"`
	User       *User     `json:"user,omitempty"`
	Mine       bool      `json:"mine"`
	Liked      bool      `json:"liked"`
}

// ToggleLikeOutput represents the output for toggle like
type ToggleLikeOutput struct {
	Liked      bool `json:"liked"`
	LikesCount int  `json:"likes_count"`
}

// CreatePost publishes a post to the user timeline and fan-ous it to his followers
func (s *Service) CreatePost(
	ctx context.Context,
	content string,
	spoilerOf *string,
	nsfw bool,
) (TimelineItem, error) {
	var ti TimelineItem
	uid, ok := ctx.Value(KeyAuthUserID).(int64)
	if !ok {
		return ti, ErrUnauthenticated
	}

	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 480 {
		return ti, ErrInvalidContent
	}

	if spoilerOf != nil {
		*spoilerOf = strings.TrimSpace(*spoilerOf)
		if *spoilerOf == "" || len([]rune(*spoilerOf)) > 64 {
			return ti, ErrInvalidSpoiler
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ti, fmt.Errorf("could not begin tx: %v", err)
	}
	defer tx.Rollback()

	query := "INSERT INTO posts (user_id, content, spoiler_of, nsfw) VALUES($1, $2, $3, $4) " +
		"RETURNING id, created_at"
	if err = tx.QueryRowContext(ctx, query, uid, content, spoilerOf, nsfw).
		Scan(&ti.Post.ID, &ti.Post.CreatedAt); err != nil {
		return ti, fmt.Errorf("could not insert post: %v", err)
	}

	ti.Post.UserID = uid
	ti.Post.Content = content
	ti.Post.SpoilerOf = spoilerOf
	ti.Post.NSFW = nsfw
	ti.Post.Mine = true

	query = "INSERT INTO timeline (user_id, post_id) VALUES ($1, $2) RETURNING id"
	if err = tx.QueryRowContext(ctx, query, uid, ti.Post.ID).Scan(&ti.ID); err != nil {
		return ti, fmt.Errorf("could not insert timeline item: %v", err)
	}

	ti.UserID = uid
	ti.PostID = ti.Post.ID

	if err = tx.Commit(); err != nil {
		return ti, fmt.Errorf("could not commit to create post: %v", err)
	}

	go func(p Post) {
		u, err := s.userByID(context.Background(), p.UserID)
		if err != nil {
			log.Printf("could not get post user: %v\n", err)
			return
		}

		p.User = &u
		p.Mine = false

		tt, err := s.fanoutPost(p)
		if err != nil {
			log.Printf("could not fanout post: %v\n", err)
			return
		}

		for _, ti = range tt {
			log.Println(litter.Sdump(ti))
			// TODO: broadcast timeline items.
		}

	}(ti.Post)

	return ti, nil
}

func (s *Service) fanoutPost(p Post) ([]TimelineItem, error) {
	query := "INSERT INTO timeline(user_id, post_id) " +
		"SELECT follower_id, $1 FROM follows WHERE followee_id = $2 " +
		"RETURNING id, user_id"
	rows, err := s.db.Query(query, p.ID, p.UserID)
	if err != nil {
		return nil, fmt.Errorf("could not insert timeline: %v", err)
	}

	defer rows.Close()

	tt := []TimelineItem{}
	for rows.Next() {
		var ti TimelineItem
		if err = rows.Scan(&ti.ID, &ti.UserID); err != nil {
			return nil, fmt.Errorf("could not scan timeline item: %v", err)
		}

		ti.PostID = p.ID
		ti.Post = p
		tt = append(tt, ti)

	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate timeline rows: %v", err)
	}

	return tt, nil
}

// TogglePostLike toggles the post like
func (s *Service) TogglePostLike(ctx context.Context, postID int64) (ToggleLikeOutput, error) {
	var out ToggleLikeOutput
	uid, ok := ctx.Value(KeyAuthUserID).(int64)
	if !ok {
		return out, ErrUnauthenticated
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("could not begin tx: %v", err)
	}

	defer tx.Rollback()

	query := `
		SELECT EXISTS (
			SELECT 1 from post_likes WHERE user_id = $1 AND post_id = $2
		)`

	if err = tx.QueryRowContext(ctx, query, uid, postID).Scan(&out.Liked); err != nil {
		return out, fmt.Errorf("could not query select post like existence: %v", err)
	}

	if out.Liked {
		query = "DELETE FROM post_likes WHERE user_id = $1 AND post_id = $2"
		if _, err = tx.ExecContext(ctx, query, uid, postID); err != nil {
			return out, fmt.Errorf("could not delete post like: %v", err)
		}

		query = "UPDATE posts SET likes_count = likes_count - 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, postID).Scan(&out.LikesCount); err != nil {
			return out, fmt.Errorf("could not update and decrement post likes count: %v", err)
		}

	} else {
		query = "INSERT INTO post_likes (user_id, post_id) VALUES ($1, $2)"
		_, err = tx.ExecContext(ctx, query, uid, postID)
		if isForeignKeyViolation(err) {
			return out, ErrPostNotFound
		}

		if err != nil {
			return out, fmt.Errorf("could not insert post like: %v", err)
		}

		query = "UPDATE posts SET likes_count = likes_count + 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, postID).Scan(&out.LikesCount); err != nil {
			return out, fmt.Errorf("could not update and increment post likes count: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("could not commit to toggle post like: %v", err)
	}

	out.Liked = !out.Liked
	return out, nil
}

// Posts from a user in descending order and with backward pagination
func (s *Service) Posts(
	ctx context.Context,
	username string,
	last int,
	before int64,
) ([]Post, error) {
	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	last = normalizePage(last)
	query, args, err := buildQuery(`
		SELECT id, content, spoiler_of, nsfw, likes_count, created_at
		{{if .auth}}
			, posts.user_id = @uid AS mine
			, likes.user_id IS NOT NULL AS liked
		{{end}}
		FROM posts
		{{if .auth}}
			LEFT JOIN post_likes as likes
				ON likes.user_id = @uid AND likes.post_id = posts.id
		{{end}}
		WHERE posts.user_id = (SELECT id FROM users WHERE username = @username)	
		{{if .before}}AND posts.id < @before{{end}}
		ORDER BY created_at DESC
		LIMIT @last
	`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
		"last":     last,
		"before":   before,
	})

	if err != nil {
		return nil, fmt.Errorf("could not build posts sql query; %v", err)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select posts: %v", err)
	}

	defer rows.Close()

	pp := make([]Post, 0, last)
	for rows.Next() {
		var p Post
		dest := []interface{}{&p.ID, &p.Content, &p.SpoilerOf, &p.NSFW, &p.LikesCount, &p.CreatedAt}
		if auth {
			dest = append(dest, &p.Mine, &p.Liked)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan post: %v", err)
		}

		pp = append(pp, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate posts rows: %v", err)
	}

	return pp, nil
}

// Post retrieves the post from a postID
func (s *Service) Post(ctx context.Context, postID int64) (Post, error) {
	var p Post

	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	query, args, err := buildQuery(`
		SELECT posts.id, content, spoiler_of, nsfw, likes_count, created_at
		, users.username, users.avatar
		{{if .auth}}
			, posts.user_id = @uid AS mine
			, likes.user_id IS NOT NULL AS liked
		{{end}}
		FROM posts
		INNER JOIN users ON posts.user_id = users.id
		{{if .auth}}
			LEFT JOIN post_likes as likes
				ON likes.user_id = @uid AND likes.post_id = posts.id
		{{end}}
		WHERE posts.id = @post_id
	`, map[string]interface{}{
		"auth":    auth,
		"uid":     uid,
		"post_id": postID,
	})

	if err != nil {
		{
			return p, fmt.Errorf("could not build post sql query: %v", err)
		}
	}

	var u User
	var avatar sql.NullString
	dest := []interface{}{&p.ID, &p.Content, &p.SpoilerOf, &p.NSFW, &p.LikesCount, &p.CreatedAt,
		&u.Username, &avatar}
	if auth {
		dest = append(dest, &p.Mine, &p.Liked)
	}

	err = s.db.QueryRowContext(ctx, query, args...).Scan(dest...)
	if err == sql.ErrNoRows {
		return p, ErrPostNotFound
	}

	if err != nil {
		return p, fmt.Errorf("could not query select post: %v", err)
	}

	if avatar.Valid {
		avatarURL := s.origin + "/img/avatars/" + avatar.String
		u.AvatarURL = &avatarURL
	}

	p.User = &u

	return p, nil
}
