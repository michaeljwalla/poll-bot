package login

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"log"
	"poll-bot/root/api/types"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var database atomic.Pointer[sql.DB]
var signer jwt.Signer
var verifier jwt.Verifier

type ServerUserData = types.ServerUserData

const QUERY_TIMEOUT = time.Duration(5000) * time.Millisecond
const DEFAULT_EXPIRY = time.Duration(15) * time.Minute

const INIT_QUERY = `
PRAGMA foreign_keys = on;

CREATE TABLE IF NOT EXISTS users (
	id     VARCHAR (24) NOT NULL,
	hash   BINARY (32)  NOT NULL,
	expiry INTEGER,
	PRIMARY KEY (
		id
	)
);

CREATE TABLE IF NOT EXISTS tokens (
	id        INTEGER       PRIMARY KEY AUTOINCREMENT,
	hash      BINARY (2048) NOT NULL,
	target_id VARCHAR (24)  NOT NULL,
	CONSTRAINT to_target FOREIGN KEY (
		target_id
	)
	REFERENCES users (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS token_lookup ON tokens (
	hash
);`

var ErrDbExists = errors.New("db already exists")
var ErrDbNotOpen = errors.New("db not open")
var ErrInvalidID = errors.New("id does not fulfill requirements")
var ErrInvalidPassword = errors.New("password does not fulfill requirements")
var ErrInvalidLogin = errors.New("unknown credentials")
var ErrInvalidToken = errors.New("the access token is invalid or expired")
var ErrNoToken = errors.New("no access token provided")

func Start(path string, signingKey []byte) error {
	if IsOpen() {
		return ErrDbExists
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	if !database.CompareAndSwap(nil, db) {
		db.Close() //nolint
		return ErrDbExists
	}
	if _, err := db.Exec(INIT_QUERY); err != nil {
		return err
	}
	signer, err = jwt.NewSignerHS(jwt.HS256, signingKey)
	if err != nil {
		db.Close() //nolint
		return err
	}
	verifier, err = jwt.NewVerifierHS(jwt.HS256, signingKey)
	if err != nil {
		db.Close() //nolint
		return err
	}
	return nil
}
func IsOpen() bool {
	return database.Load() != nil
}

// expects valid body
func newServerUserData(body *LoginBody, expiry time.Duration) *ServerUserData {
	hash := sha256.Sum256([]byte(body.Password))
	return &ServerUserData{
		ID:           strings.ToLower(body.ID),
		PasswordHash: hash[:],
		Expiry:       expiry,
	}
}

func buildReplacer(numRows int) (msg string, err error) {
	const QUERY = `REPLACE INTO users(id, hash, expiry) VALUES `
	if numRows < 1 {
		return "", errors.New("must add at least one entry")
	}
	builder := strings.Builder{}
	builder.WriteString(QUERY)
	for i := range numRows {
		builder.WriteString("(?, ?, ?)")
		if i == numRows-1 {
			builder.WriteString(";")
		} else {
			builder.WriteString(", ")
		}
	}
	return builder.String(), nil
}
func flatten(data []*ServerUserData) []any {
	const FIELDS = 3
	result := make([]any, 0, len(data)*FIELDS)
	for _, row := range data {
		result = append(result, row.ID, row.PasswordHash, row.Expiry)
	}
	return result
}

func replaceRows(data []*ServerUserData) (sql.Result, error) {
	if !IsOpen() {
		log.Panic(ErrDbNotOpen)
	}
	query, err := buildReplacer(len(data))
	if err != nil {
		return nil, err
	}

	db := database.Load()
	ctx, cancel := context.WithTimeout(context.Background(), QUERY_TIMEOUT)
	defer cancel()

	return db.ExecContext(ctx, query, flatten(data)...)
}

// expects validated. ErrNoRows
func getUserFromLogin(body *LoginBody) (*ServerUserData, error) {
	if !IsOpen() {
		log.Panic(ErrDbNotOpen)
	}
	const QUERY = `
SELECT expiry 
  FROM users
 WHERE id = ? AND
	   hash = ?
 LIMIT 1;`
	db := database.Load()
	ctx, cancel := context.WithTimeout(context.Background(), QUERY_TIMEOUT)
	defer cancel()

	hash := sha256.Sum256([]byte(body.Password))
	row := db.QueryRowContext(ctx, QUERY, body.ID, hash[:])
	var expiry sql.NullInt64
	if err := row.Scan(&expiry); err != nil {
		return nil, err //errnorows
	}
	if expiry.Valid && expiry.Int64 > 0 {
		return newServerUserData(body, time.Duration(expiry.Int64)), nil
	}
	return newServerUserData(body, DEFAULT_EXPIRY), nil
}

func newTokenForUser(data *ServerUserData) (*jwt.Token, error) {
	builder := jwt.NewBuilder(signer)
	unixNow := time.Now().UTC()
	claims := &jwt.RegisteredClaims{
		ID:        uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(unixNow),
		ExpiresAt: jwt.NewNumericDate(unixNow.Add(data.Expiry)),
	}
	return builder.Build(claims)
}

func ValidateToken(raw []byte) bool {
	var claims jwt.RegisteredClaims
	if err := jwt.ParseClaims(raw, verifier, &claims); err != nil {
		return false
	}
	return claims.IsValidAt(time.Now())
}

// TokenExpiry reports when a valid token lapses. The session cookie is
// HttpOnly, so a browser client cannot read the claim itself; this is how the
// page learns how long it has left without the raw token being handed to JS.
func TokenExpiry(raw []byte) (time.Time, bool) {
	var claims jwt.RegisteredClaims
	if err := jwt.ParseClaims(raw, verifier, &claims); err != nil {
		return time.Time{}, false
	}
	if !claims.IsValidAt(time.Now()) || claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time, true
}

// all user creation / modifications are done through bot, no public rest for them
func ModifyUser(id string, pass string, expiry time.Duration) error {
	if !IsOpen() {
		log.Panic(ErrDbNotOpen)
	} else if !isValidID(id) {
		return ErrInvalidID
	} else if !isValidPass(pass) {
		return ErrInvalidPassword
	}
	hash := sha256.Sum256([]byte(pass))
	entry := ServerUserData{
		ID:           id,
		PasswordHash: hash[:],
	}
	if expiry > 0 {
		entry.Expiry = expiry
	} //else no field = default
	_, err := replaceRows([]*ServerUserData{&entry})
	return err
}

func DropUser(id string) error {
	if !IsOpen() {
		log.Panic(ErrDbNotOpen)
	} else if !isValidID(id) {
		return ErrInvalidID
	}
	const QUERY = `DELETE FROM users WHERE id = ?;`

	db := database.Load()
	ctx, cancel := context.WithTimeout(context.Background(), QUERY_TIMEOUT)
	defer cancel()

	_, err := db.ExecContext(ctx, QUERY, id)
	return err
}
