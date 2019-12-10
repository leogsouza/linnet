package service

import (
	"image/jpeg"
	"image/png"
	"os"
	"path"

	"github.com/disintegration/imaging"
	gonanoid "github.com/matoous/go-nanoid"

	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"regexp"
	"strings"
)

// MaxAvatarBytes maximum file upload size
const MaxAvatarBytes = 5 << 20 // 5MB

var (
	rxEmail        = regexp.MustCompile("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")
	rxUsername     = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_-]{0,17}$")
	rxAvatarFormat = regexp.MustCompile("^(png|jpg|jpeg)$")
	avatarsDir     = path.Join("web", "static", "img", "avatars")
)

var (
	// ErrUserNotFound used when the user wasn't found on the db.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidEmail used when the email is invalid.
	ErrInvalidEmail = errors.New("invalid email")
	// ErrInvalidUsername used when the username is invalid.
	ErrInvalidUsername = errors.New("invalid username")
	// ErrEmailTaken used when there is already an user registered with that email
	ErrEmailTaken = errors.New("email already exists")
	// ErrUsernameTaken used when there is already an user registered with that username
	ErrUsernameTaken = errors.New("username already exists")
	// ErrForbiddenFollow used when you try to follow yourself
	ErrForbiddenFollow = errors.New("cannot follow yourself")
	// ErrUnsupportedAvatarFormat used when the avatar file extension is invalid
	ErrUnsupportedAvatarFormat = errors.New("only png and jpeg allowed as avatar")
)

// User model
type User struct {
	ID        int64   `json:"id,omitempty"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatar_url"`
}

// UserProfile model
type UserProfile struct {
	User
	Email          string `json:"email,omitempty"`
	FollowersCount int    `json:"followers_count"`
	FolloweesCount int    `json:"followees_count"`
	Me             bool   `json:"me"`
	Following      bool   `json:"following"`
	Followeed      bool   `json:"followed"`
}

// ToggleFollowOutput response
type ToggleFollowOutput struct {
	Following      bool `json:"following"`
	FollowersCount int  `json:"followers_count"`
}

// CreateUser inserts an user in the database
func (s *Service) CreateUser(ctx context.Context, email, username string) error {

	email = strings.TrimSpace(email)
	if !rxEmail.MatchString(email) {
		return ErrInvalidEmail
	}

	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return ErrInvalidUsername
	}

	query := "INSERT INTO users (email, username) VALUES ($1, $2)"
	_, err := s.db.ExecContext(ctx, query, email, username)

	unique := isUniqueViolation(err)

	if unique && strings.Contains(err.Error(), "email") {
		return ErrEmailTaken
	}

	if unique && strings.Contains(err.Error(), "username") {
		return ErrUsernameTaken
	}

	if err != nil {
		return fmt.Errorf("could not insert user: %v", err)
	}

	return nil
}

// Users in ascending order with foward pagination and filtered by username.
func (s *Service) Users(ctx context.Context, search string, first int, after string) ([]UserProfile, error) {
	search = strings.TrimSpace(search)
	after = strings.TrimSpace(after)
	first = normalizePage(first)
	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	query, args, err := buildQuery(`
		SELECT id, email, username, avatar, followers_count, followees_count
		{{if .auth}}
			, followers.follower_id IS NOT NULL as following
			, followees.followee_id IS NOT NULL as followeed
		{{end}}
		FROM users
		{{if .auth}}
			LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
			LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{end}}
		{{if or .search .after}}WHERE{{end}}
		{{if .search}}username ILIKE '%' || @search || '%'{{end}}
		{{if and .search .after}}AND{{end}}
		{{if .after}}username > @after{{end}}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":   auth,
		"uid":    uid,
		"search": search,
		"first":  first,
		"after":  after,
	})

	if err != nil {
		return nil, fmt.Errorf("could not build users sql query: %v", err)
	}

	log.Printf("users query: %s\nargs: %v\n", query, args)

	rows, err := s.db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, fmt.Errorf("could not query select users: %v", err)
	}

	defer rows.Close()
	uu := make([]UserProfile, 0, first)

	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FollowersCount, &u.FolloweesCount}

		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan user: %v", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = 0
			u.Email = ""
		}

		if avatar.Valid {
			avatarURL := s.origin + "/img/avatars/" + avatar.String
			u.AvatarURL = &avatarURL
		}

		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate user rows: %v", err)
	}

	return uu, nil
}

func (s *Service) userByID(ctx context.Context, id int64) (User, error) {
	var u User
	var avatar sql.NullString
	query := "SELECT username,avatar FROM users where id = $1"
	err := s.db.QueryRowContext(ctx, query, id).Scan(&u.Username, &avatar)
	if err == sql.ErrNoRows {
		return u, ErrUserNotFound
	}

	if err != nil {
		return u, fmt.Errorf("could not query select auth user: %v", err)
	}

	u.ID = id
	if avatar.Valid {
		avatarURL := s.origin + "/img/avatars/" + avatar.String
		u.AvatarURL = &avatarURL
	}

	return u, nil
}

// User selects the user from the database with the given username
func (s *Service) User(ctx context.Context, username string) (UserProfile, error) {

	var u UserProfile

	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return u, ErrInvalidUsername
	}

	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	args := []interface{}{username}

	var avatar sql.NullString
	dest := []interface{}{&u.ID, &u.Email, &avatar, &u.FolloweesCount, &u.FolloweesCount}

	query := "SELECT id, email, avatar, followers_count, followees_count "
	if auth {
		query += ", " +
			"followers.follower_id IS NOT NULL AS following, " +
			"followees.followee_id IS NOT NULL AS followeed "
		dest = append(dest, &u.Following, &u.Followeed)
	}
	query += "FROM users "
	if auth {
		query += "LEFT JOIN follows AS followers on followers.follower_id = $2 and followers.followee_id = users.id " +
			"LEFT JOIN follows AS followees on followees.follower_id = users.id AND followees.followee_id = $2 "
		args = append(args, uid)
	}
	query += "WHERE username = $1"
	err := s.db.QueryRowContext(ctx, query, args...).Scan(dest...)

	if err == sql.ErrNoRows {
		return u, ErrUserNotFound
	}

	if err != nil {
		return u, fmt.Errorf("could not query select user: %v", err)
	}

	u.Username = username
	u.Me = auth && uid == u.ID
	if !u.Me {
		u.ID = 0
		u.Email = ""
	}

	if avatar.Valid {
		avatarURL := s.origin + "/img/avatars/" + avatar.String
		u.AvatarURL = &avatarURL
	}

	return u, nil

}

// UpdateAvatar updates the avatar of an authenticated user
func (s *Service) UpdateAvatar(ctx context.Context, r io.Reader) (string, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(int64)
	if !ok {
		return "", ErrUnauthenticated
	}
	r = io.LimitReader(r, MaxAvatarBytes)
	img, format, err := image.Decode(r)
	if err != nil {
		return "", fmt.Errorf("could not read avatar: %v", err)
	}

	if !rxAvatarFormat.MatchString(format) {
		return "", ErrUnsupportedAvatarFormat
	}

	avatar, err := gonanoid.Nanoid()
	if err != nil {
		return "", fmt.Errorf("could not generate avatar filename: %v", err)
	}

	if format == "png" {
		avatar += ".png"
	} else {
		avatar += ".jpg"
	}

	err = createDirectory(avatarsDir)
	if err != nil {
		return "", fmt.Errorf("could not create avatar directory: %v", err)
	}

	avatarPath := path.Join(avatarsDir, avatar)
	f, err := os.Create(avatarPath)
	if err != nil {
		return "", fmt.Errorf("could not create avatar file: %v", err)
	}

	defer f.Close()
	img = imaging.Fill(img, 400, 400, imaging.Center, imaging.CatmullRom)
	if format == "png" {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, nil)
	}

	if err != nil {
		return "", fmt.Errorf("could not write avatar to disk: %v", err)
	}

	var oldAvatar sql.NullString
	if err = s.db.QueryRowContext(ctx, `
		UPDATE users SET avatar = $1 WHERE id = $2
		RETURNING (SELECT avatar FROM users WHERE id = $2) AS old_avatar
	`, avatar, uid).Scan(&oldAvatar); err != nil {
		defer os.Remove(avatarPath)
		return "", fmt.Errorf("could not update avatar: %v", err)
	}

	if oldAvatar.Valid {
		defer os.Remove(path.Join(avatarsDir, oldAvatar.String))
	}

	return s.origin + "/img/avatars/" + avatar, nil

}

// ToggleFollow between two users
func (s *Service) ToggleFollow(ctx context.Context, username string) (ToggleFollowOutput, error) {
	var out ToggleFollowOutput

	followerID, ok := ctx.Value(KeyAuthUserID).(int64)

	if !ok {
		return out, ErrUnauthenticated
	}

	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return out, ErrInvalidUsername
	}

	tx, err := s.db.BeginTx(ctx, nil)

	if err != nil {
		return out, fmt.Errorf("could not begin tx: %v", err)
	}

	defer tx.Rollback()

	var followeeID int64

	query := "SELECT id FROM users WHERE username = $1"
	err = tx.QueryRowContext(ctx, query, username).Scan(&followeeID)

	if err == sql.ErrNoRows {
		return out, ErrUserNotFound
	}

	if err != nil {
		return out, fmt.Errorf("could not query select user id from followeee username: %v", err)
	}

	if followeeID == followerID {
		return out, ErrForbiddenFollow
	}

	query = "SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)"
	if err = tx.QueryRowContext(ctx, query, followerID, followeeID).Scan(&out.Following); err != nil {
		return out, fmt.Errorf("could not query select existance of follow: %v", err)
	}

	if out.Following {
		query = "DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2"
		if _, err = tx.ExecContext(ctx, query, followerID, followeeID); err != nil {
			return out, fmt.Errorf("could not delete follow: %v", err)
		}

		query = "UPDATE users SET followees_count = followees_count - 1 WHERE id = $1"
		if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
			return out, fmt.Errorf("could not update follower followees count (-): %v", err)
		}

		query = "UPDATE users SET followers_count = followers_count - 1 WHERE id = $1 RETURNING followers_count"
		if err = tx.QueryRowContext(ctx, query, followeeID).Scan(&out.FollowersCount); err != nil {
			return out, fmt.Errorf("could not update followee followers count (-): %v", err)
		}
	} else {
		query = "INSERT INTO follows (follower_id, followee_id) VALUES($1, $2)"
		if _, err = tx.ExecContext(ctx, query, followerID, followeeID); err != nil {
			return out, fmt.Errorf("could not insert follow: %v", err)
		}

		query = "UPDATE users SET followees_count = followees_count + 1 WHERE id = $1"
		if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
			return out, fmt.Errorf("could not update follower followees count (+): %v", err)
		}

		query = "UPDATE users SET followers_count = followers_count + 1 WHERE id = $1 RETURNING followers_count"
		if err = tx.QueryRowContext(ctx, query, followeeID).Scan(&out.FollowersCount); err != nil {
			return out, fmt.Errorf("could not update followee followers count (+): %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("could not commit toggle follow: %v", err)
	}

	out.Following = !out.Following

	if out.Following {
		// TODO: notify followee.
	}

	return out, nil
}

// Followers in ascending order with foward pagination and filtered by username.
func (s *Service) Followers(ctx context.Context, username string, first int, after string) ([]UserProfile, error) {
	username = strings.TrimSpace(username)

	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	after = strings.TrimSpace(after)
	first = normalizePage(first)
	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	query, args, err := buildQuery(`
		SELECT id, email, username, avatar, followers_count, followees_count
		{{if .auth}}
			, followers.follower_id IS NOT NULL as following
			, followees.followee_id IS NOT NULL as followeed
		{{end}}
		FROM follows
		INNER JOIN users ON follows.follower_id = users.id
		{{if .auth}}
			LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
			LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{end}}
		WHERE follows.followee_id = (SELECT id FROM users WHERE username = @username)
		{{if .after}}username > @after{{end}}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
		"first":    first,
		"after":    after,
	})

	if err != nil {
		return nil, fmt.Errorf("could not build followers sql query: %v", err)
	}

	log.Printf("users query: %s\nargs: %v\n", query, args)

	rows, err := s.db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, fmt.Errorf("could not query select followers: %v", err)
	}

	defer rows.Close()
	uu := make([]UserProfile, 0, first)
	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FollowersCount, &u.FolloweesCount}

		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan follower: %v", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = 0
			u.Email = ""
		}

		if avatar.Valid {
			avatarURL := s.origin + "/img/avatars/" + avatar.String
			u.AvatarURL = &avatarURL
		}

		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate follower rows: %v", err)
	}

	return uu, nil
}

// Followees in ascending order with foward pagination and filtered by username.
func (s *Service) Followees(ctx context.Context, username string, first int, after string) ([]UserProfile, error) {
	username = strings.TrimSpace(username)

	username = strings.TrimSpace(username)
	if !rxUsername.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	after = strings.TrimSpace(after)
	first = normalizePage(first)
	uid, auth := ctx.Value(KeyAuthUserID).(int64)
	query, args, err := buildQuery(`
		SELECT id, email, username, avatar, followers_count, followees_count
		{{if .auth}}
			, followers.follower_id IS NOT NULL as following
			, followees.followee_id IS NOT NULL as followeed
		{{end}}
		FROM follows
		INNER JOIN users ON follows.followee_id = users.id
		{{if .auth}}
			LEFT JOIN follows AS followers ON followers.follower_id = @uid AND followers.followee_id = users.id
			LEFT JOIN follows AS followees ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{end}}
		WHERE follows.follower_id = (SELECT id FROM users WHERE username = @username)
		{{if .after}}username > @after{{end}}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
		"first":    first,
		"after":    after,
	})

	if err != nil {
		return nil, fmt.Errorf("could not build followees sql query: %v", err)
	}

	log.Printf("followees query: %s\nargs: %v\n", query, args)

	rows, err := s.db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, fmt.Errorf("could not query select followees: %v", err)
	}

	defer rows.Close()
	uu := make([]UserProfile, 0, first)
	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{&u.ID, &u.Email, &u.Username, &avatar, &u.FollowersCount, &u.FolloweesCount}

		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan followee: %v", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = 0
			u.Email = ""
		}

		if avatar.Valid {
			avatarURL := s.origin + "/img/avatars/" + avatar.String
			u.AvatarURL = &avatarURL
		}

		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate followee rows: %v", err)
	}

	return uu, nil
}
