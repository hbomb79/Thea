package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
)

var ErrUserNotFound = errors.New("user does not exist")

var log = logger.Get("UserStore")

type (
	User struct {
		ID             uuid.UUID  `db:"id"`
		Username       string     `db:"username"`
		HashedPassword []byte     `db:"password" json:"-"`
		HashSalt       []byte     `db:"salt" json:"-"`
		CreatedAt      time.Time  `db:"created_at"`
		UpdatedAt      time.Time  `db:"updated_at"`
		LastLoginAt    *time.Time `db:"last_login"`
		LastRefreshAt  *time.Time `db:"last_refresh"`
		//TODO Permissions []string
	}

	Store struct {
		hasher *argonHasher
	}
)

func NewStore() *Store {
	return &Store{
		//TODO figure out the best values for this
		newArgon2IdHasher(1, 64, 64*1024, 1, 128),
	}
}

func (store *Store) Create(db database.Queryable, username []byte, rawPassword []byte) error {
	hash, err := store.hasher.GenerateHash(rawPassword, []byte{})
	if err != nil {
		return fmt.Errorf("provided password is invalid: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO users(id, username, password, salt, created_at, updated_at, last_login, last_refresh)
		VALUES ($1, $2, $3, $4, current_timestamp, current_timestamp, NULL, NULL)
	`, uuid.New(), username, hash.hash, hash.salt)
	if err != nil {
		return fmt.Errorf("failed to insert new user: %w", err)
	}

	return nil
}

func (store *Store) List(db database.Queryable) ([]*User, error) {
	var results []*User
	if err := db.Select(&results, `SELECT * FROM users`); err != nil {
		return nil, err
	}

	return results, nil
}

// GetWithUsernameAndPassword finds a user with the matching
// username and returns it IF and ONLY IF the raw (unhashed) password
// provided is able to be hashed with the same salt as was used with
// the existing user (if any), and the hashes MATCH.
func (store *Store) GetWithUsernameAndPassword(db database.Queryable, username []byte, rawPassword []byte) (*User, error) {
	var user User
	if err := db.Get(&user, `SELECT * FROM users WHERE username=$1`, username); err != nil {
		return nil, fmt.Errorf("failed to find user with username %s: %w", username, err)
	}

	if err := store.hasher.Compare(user.HashedPassword, user.HashSalt, rawPassword); err != nil {
		return nil, fmt.Errorf("password supplied for user %s is invalid: %v", username, err)
	}

	return &user, nil
}

func (store *Store) GetWithId(db database.Queryable, id uuid.UUID) (*User, error) {
	var user User
	if err := db.Get(&user, `SELECT * FROM users WHERE id=$1`, id); err != nil {
		return nil, ErrUserNotFound
	}

	return &user, nil
}

func (store *Store) RecordLogin(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`UPDATE users SET last_login=current_timestamp WHERE id = $1`, userID)
	return err
}

func (store *Store) RecordRefresh(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`UPDATE users SET last_refresh=current_timestamp WHERE id = $1`, userID)
	return err
}

//TODO func (store *store) UpdatePermissions(username string, permissions []string) error {}
