package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"github.com/leebrouse/greenLight/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

// Declare a new AnonymousUser variable.
var AnonymousUser = &User{}

/*
	data.AnonymousUser.IsAnonymous() // → Returns true
	otherUser := &data.User{}
	otherUser.IsAnonymous() // → Returns false
*/
// Check if a User instance is the AnonymousUser.
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	Version   int       `json:"-"`
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash
	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	if err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword)); err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

/*Validate check*/

// ValidateEmail
func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRegex), "email", "must be a valid email address")
}

// ValidatePassword
func ValidatePassword(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

// ValidateUser
func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePassword(v, *user.Password.plaintext)
	}

	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

/* UserModel*/
var (
	//email should be unique
	ErrDuplicateEmail = errors.New("duplicate email")
)

type UserModel struct {
	DB *sql.DB
}

func (m *UserModel) Insert(user *User) error {
	query := `
				INSERT INTO users (name, email, password_hash, activated)
				VALUES ($1, $2, $3, $4)
				RETURNING id, created_at, version
			`
	arg := []interface{}{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, arg...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}

	}
	return nil
}

func (m *UserModel) GetByEmail(email string) (*User, error) {

	var user User

	query := `
				SELECT id, created_at, name, email, password_hash, activated, version
				FROM users
				WHERE email = $1
			`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			//use ErrRecordNotFound due to indeed,can't find the actual row
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (m *UserModel) Update(user *User) error {

	query := `
				UPDATE users
				SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
				WHERE id = $5 AND version = $6
				RETURNING version
			`
	args := []interface{}{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			//use ErrEditConflict due to race competition
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

// A submethod for retrieving the user by the given token
func (m UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	//get the tokenHash using the tokenPlaintext
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	//sql
	query := `
				SELECT users.id, users.created_at, users.name, users.email, users.password_hash, users.activated, users.version
				FROM users
				INNER JOIN tokens
				ON users.id = tokens.user_id
				WHERE tokens.hash = $1
				AND tokens.scope = $2
				AND tokens.expiry > $3
			`
	//arguments in the sql query
	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	//create the context
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//user
	var user User

	//find the suit user
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}
