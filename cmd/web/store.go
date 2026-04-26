package main

import (
	"time"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

type dataStore interface {
	MigrateUp() error
	GetActiveFiscalYear() (database.FiscalYear, bool, error)
	GetISRBrackets(fiscalYearID int) ([]database.ISRBracket, error)
	GetIMSSConcepts() ([]database.IMSSConcept, error)
	GetCesantiaBracket(fiscalYearID int, salaryInUMAs float64) (database.CesantiaBracket, bool, error)
	GetRESICOBracket(fiscalYearID int, monthlyIncome float64) (database.RESICOBracket, bool, error)
	InsertUser(email, hashedPassword string) (int, error)
	GetUser(id int) (database.User, bool, error)
	GetUserByEmail(email string) (database.User, bool, error)
	UpdateUserHashedPassword(id int, hashedPassword string) error
	UpdateUserAPIKey(id int, apiKey string) error
	GetUserByAPIKey(apiKey string) (database.User, bool, error)
	IncrementAPICallsCount(id int) error
	GetDailyAPICallCount(userID int) (int, error)
	LogAPICall(userID int) error
	InsertEmailVerificationToken(userID int, hashedToken string) error
	GetUserIDFromVerificationToken(hashedToken string) (int, bool, error)
	VerifyUserEmail(userID int) error
	DeleteEmailVerificationTokensForUser(userID int) error
	InsertPasswordReset(hashedToken string, userID int, ttl time.Duration) error
	GetPasswordReset(hashedToken string) (database.PasswordReset, bool, error)
	DeletePasswordResets(userID int) error
}
