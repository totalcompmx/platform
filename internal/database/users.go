package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID              int            `db:"id"`
	Created         time.Time      `db:"created"`
	Email           string         `db:"email"`
	HashedPassword  string         `db:"hashed_password"`
	ApiKey          sql.NullString `db:"api_key"`
	ApiCallsCount   int            `db:"api_calls_count"`
	ApiKeyCreatedAt sql.NullTime   `db:"api_key_created_at"`
	EmailVerified   bool           `db:"email_verified"`
	EmailVerifiedAt sql.NullTime   `db:"email_verified_at"`
}

func (db *DB) InsertUser(email, hashedPassword string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var id int

	query := `
		INSERT INTO users (created, email, hashed_password)
		VALUES ($1, $2, $3)
		RETURNING id`

	err := db.GetContext(ctx, &id, query, time.Now(), email, hashedPassword)
	if err != nil {
		return 0, err
	}

	return id, err
}

func (db *DB) GetUser(id int) (User, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var user User

	query := `SELECT * FROM users WHERE id = $1`

	err := db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}

	return user, true, err
}

func (db *DB) GetUserByEmail(email string) (User, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var user User

	query := `SELECT * FROM users WHERE LOWER(email) = LOWER($1)`

	err := db.GetContext(ctx, &user, query, email)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}

	return user, true, err
}

func (db *DB) UpdateUserHashedPassword(id int, hashedPassword string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `UPDATE users SET hashed_password = $1 WHERE id = $2`

	_, err := db.ExecContext(ctx, query, hashedPassword, id)
	return err
}

// UpdateUserAPIKey updates or creates an API key for a user
func (db *DB) UpdateUserAPIKey(id int, apiKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `
		UPDATE users 
		SET api_key = $1, api_key_created_at = $2 
		WHERE id = $3`

	_, err := db.ExecContext(ctx, query, apiKey, time.Now(), id)
	return err
}

// GetUserByAPIKey retrieves a user by their API key
func (db *DB) GetUserByAPIKey(apiKey string) (User, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var user User

	query := `SELECT * FROM users WHERE api_key = $1`

	err := db.GetContext(ctx, &user, query, apiKey)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}

	return user, true, err
}

// IncrementAPICallsCount increments the API calls counter for a user
func (db *DB) IncrementAPICallsCount(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `UPDATE users SET api_calls_count = api_calls_count + 1 WHERE id = $1`

	_, err := db.ExecContext(ctx, query, id)
	return err
}

// GetDailyAPICallCount returns how many API calls a user has made today
// This is used to enforce rate limits for unverified users (10/day)
func (db *DB) GetDailyAPICallCount(userID int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var count int

	// Count rows from api_call_logs table for today
	// We'll create this table in a moment
	query := `
		SELECT COUNT(*) 
		FROM api_call_logs 
		WHERE user_id = $1 
		AND created_at >= CURRENT_DATE`

	err := db.GetContext(ctx, &count, query, userID)
	return count, err
}

// LogAPICall records an API call for rate limiting purposes
func (db *DB) LogAPICall(userID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `INSERT INTO api_call_logs (user_id, created_at) VALUES ($1, NOW())`

	_, err := db.ExecContext(ctx, query, userID)
	return err
}

// InsertEmailVerificationToken stores a verification token for a user
func (db *DB) InsertEmailVerificationToken(userID int, hashedToken string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `
		INSERT INTO email_verifications (user_id, hashed_token, created)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) 
		DO UPDATE SET hashed_token = $2, created = $3`

	_, err := db.ExecContext(ctx, query, userID, hashedToken, time.Now())
	return err
}

// GetUserIDFromVerificationToken retrieves user ID and validates token hasn't expired (24 hours)
func (db *DB) GetUserIDFromVerificationToken(hashedToken string) (int, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var userID int

	query := `
		SELECT user_id 
		FROM email_verifications 
		WHERE hashed_token = $1 
		AND created > NOW() - INTERVAL '24 hours'`

	err := db.GetContext(ctx, &userID, query, hashedToken)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}

	return userID, true, err
}

// VerifyUserEmail marks a user's email as verified and deletes the verification token
func (db *DB) VerifyUserEmail(userID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark email as verified
	query1 := `UPDATE users SET email_verified = TRUE, email_verified_at = $1 WHERE id = $2`
	_, err = tx.ExecContext(ctx, query1, time.Now(), userID)
	if err != nil {
		return err
	}

	// Delete verification token
	query2 := `DELETE FROM email_verifications WHERE user_id = $1`
	_, err = tx.ExecContext(ctx, query2, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteEmailVerificationTokensForUser deletes all verification tokens for a user
func (db *DB) DeleteEmailVerificationTokensForUser(userID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	query := `DELETE FROM email_verifications WHERE user_id = $1`
	_, err := db.ExecContext(ctx, query, userID)
	return err
}
