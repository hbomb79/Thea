package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/hbomb79/Thea/internal/database"
	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
)

var ErrUserNotFound = errors.New("user does not exist")

var log = logger.Get("UserStore")

type (
	Permissions []string

	userBase struct {
		ID             uuid.UUID  `db:"id"`
		Username       string     `db:"username"`
		HashedPassword []byte     `db:"password" json:"-"`
		HashSalt       []byte     `db:"salt" json:"-"`
		CreatedAt      time.Time  `db:"created_at"`
		UpdatedAt      time.Time  `db:"updated_at"`
		LastLoginAt    *time.Time `db:"last_login"`
		LastRefreshAt  *time.Time `db:"last_refresh"`
	}

	// userModel is a combination of the users table columns, combined with
	// a JSON representation of the coalesced permission rows which are
	// joined in to the query. We use a separate struct as part of
	// the public API of this store to hide the use of the JsonColumn container
	// to prevent against breakages if we change this in the future
	userModel struct {
		userBase
		Permissions database.JsonColumn[[]string] `db:"permissions"`
	}

	// User is the external/public API for the user model. It uses a special
	// Permissions type for the users permissions which allows for common
	// operations to be performed against the set of permissions
	User struct {
		userBase
		Permissions Permissions
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
	query, args, err := selectUserBuilder().ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to construct list users query: %w", err)
	}

	var results []userModel
	if err := db.Select(&results, query, args...); err != nil {
		return nil, err
	}

	output := make([]*User, len(results))
	for k, v := range results {
		output[k] = userModelToUser(&v)
	}

	return output, nil
}

// GetWithUsernameAndPassword finds a user with the matching
// username and returns it IF and ONLY IF the raw (unhashed) password
// provided is able to be hashed with the same salt as was used with
// the existing user (if any), and the hashes MATCH.
func (store *Store) GetWithUsernameAndPassword(db database.Queryable, username []byte, rawPassword []byte) (*User, error) {
	query, args, err := selectUserBuilder().Where("users.username=?", username).ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to construct select user query: %w", err)
	}

	var user userModel
	if err := db.Get(&user, db.Rebind(query), args...); err != nil {
		return nil, fmt.Errorf("failed to find user with username %s: %w", username, err)
	}

	if err := store.hasher.Compare(user.HashedPassword, user.HashSalt, rawPassword); err != nil {
		return nil, fmt.Errorf("password supplied for user %s is invalid: %v", username, err)
	}

	return userModelToUser(&user), nil
}

func (store *Store) GetWithId(db database.Queryable, id uuid.UUID) (*User, error) {
	query, args, err := selectUserBuilder().Where("users.id=?", id).ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to construct select user query: %w", err)
	}

	var user userModel
	if err := db.Get(&user, db.Rebind(query), args...); err != nil {
		return nil, ErrUserNotFound
	}

	return userModelToUser(&user), nil
}

func (store *Store) RecordUpdate(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`UPDATE users SET updated_at=current_timestamp WHERE id = $1`, userID)
	return err
}

func (store *Store) RecordLogin(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`UPDATE users SET last_login=current_timestamp WHERE id = $1`, userID)
	return err
}

func (store *Store) RecordRefresh(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`UPDATE users SET last_refresh=current_timestamp WHERE id = $1`, userID)
	return err
}

func (store *Store) DropUserPermissions(db database.Queryable, userID uuid.UUID) error {
	_, err := db.Exec(`DELETE FROM users_permissions WHERE user_id=$1`, userID)
	return err
}

type Permission struct {
	ID    uuid.UUID `db:"id"`
	Label string    `db:"label"`
}

func (store *Store) GetPermissionsByLabel(db database.Queryable, permissionLabels []string) ([]Permission, error) {
	var results []Permission
	query, args, err := sqlx.In(`SELECT * FROM permissions WHERE label IN (?)`, permissionLabels)
	if err != nil {
		return nil, err
	}

	if err := db.Select(&results, db.Rebind(query), args...); err != nil {
		return nil, err
	}

	return results, nil
}

func (store *Store) InsertUserPermissions(db database.Queryable, userID uuid.UUID, permissions []Permission) error {
	_, err := db.NamedExec(`
		INSERT INTO users_permissions(user_id, permission_id)
		VALUES('`+userID.String()+`', :id)
		ON CONFLICT(user_id, permission_id) DO NOTHING
	`, permissions)
	return err
}

func selectUserBuilder() squirrel.SelectBuilder {
	return squirrel.
		Select("users.*", "COALESCE(JSONB_AGG(DISTINCT permissions.label) FILTER (WHERE permissions.id IS NOT NULL), '[]') AS permissions").
		From("users").
		LeftJoin("users_permissions ON users_permissions.user_id = users.id").
		LeftJoin("permissions ON permissions.id = users_permissions.permission_id").
		GroupBy("users.id")
}

func userModelToUser(model *userModel) *User {
	return &User{
		userBase:    model.userBase,
		Permissions: *model.Permissions.Get(),
	}
}
