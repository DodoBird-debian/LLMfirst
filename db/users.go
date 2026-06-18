package db

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	PasswordHash string `json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HasUsers checks if there are any registered users.
func HasUsers(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateUser registers a new user. The first user always becomes an admin.
func CreateUser(db *sql.DB, username, password string) (*User, error) {
	hasUsers, err := HasUsers(db)
	if err != nil {
		return nil, err
	}

	role := "user"
	if !hasUsers {
		role = "admin"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := db.Exec("INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)", username, string(hash), role)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Username:  username,
		Role:      role,
		CreatedAt: time.Now(),
	}, nil
}

// CreateUserByAdmin registers a new user created by the admin, forcing the 'user' role.
func CreateUserByAdmin(db *sql.DB, username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := db.Exec("INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)", username, string(hash), "user")
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Username:  username,
		Role:      "user",
		CreatedAt: time.Now(),
	}, nil
}

// AuthenticateUser verifies the username and password, returning the User object if successful.
func AuthenticateUser(db *sql.DB, username, password string) (*User, error) {
	var u User
	row := db.QueryRow("SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?", username)
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid username or password")
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	return &u, nil
}

// GetUserByID fetches a user by their unique ID.
func GetUserByID(db *sql.DB, id int64) (*User, error) {
	var u User
	row := db.QueryRow("SELECT id, username, role, created_at FROM users WHERE id = ?", id)
	err := row.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUsers retrieves a list of all users.
func GetUsers(db *sql.DB) ([]User, error) {
	rows, err := db.Query("SELECT id, username, role, created_at FROM users ORDER BY username ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// DeleteUser deletes a user by their ID.
func DeleteUser(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// CreateSession generates a new session token for the user.
func CreateSession(db *sql.DB, userID int64, duration time.Duration) (*Session, error) {
	token := uuid.New().String()
	expiresAt := time.Now().Add(duration)

	_, err := db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	if err != nil {
		return nil, err
	}

	return &Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateSession checks the token validity and returns the User object if valid.
func ValidateSession(db *sql.DB, token string) (*User, error) {
	var u User
	var expiresAt time.Time
	err := db.QueryRow(`
		SELECT u.id, u.username, u.role, u.created_at, s.expires_at 
		FROM sessions s 
		JOIN users u ON s.user_id = u.id 
		WHERE s.token = ?`, token).Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	if time.Now().After(expiresAt) {
		_ = DeleteSession(db, token)
		return nil, errors.New("session expired")
	}

	return &u, nil
}

// DeleteSession removes a session token from the database.
func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}
