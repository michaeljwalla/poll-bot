package login

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"poll-bot/root/api/general"
	"poll-bot/root/api/types"
	"regexp"

	"github.com/cristalhq/jwt/v5"
)

var newErrResponse = general.NewErrResponse

type LoginBody = types.LoginBody
type LoginResponse = types.LoginResponse
type UserData = types.UserData

const ID_MIN_LENGTH = 3
const ID_MAX_LENGTH = 24

const PASS_MIN_LENGTH = 8
const PASS_MAX_LENGTH = 128

var ID_RULES = map[string]bool{
	`[a-zA-Z_]`:  true,
	`^[^_]`:      true,
	`[^_]$`:      true,
	`[^a-zA-Z_]`: false,
}
var PASS_RULES = map[string]bool{
	`[a-z]`:        true,
	`[A-Z]`:        true,
	`[0-9]`:        true,
	`[^a-zA-Z0-9]`: true,
}

type matchRule = struct {
	check   *regexp.Regexp
	present bool
}
type matchMap = map[string]matchRule

var matchers struct {
	ID       matchMap
	Password matchMap
}

// validate rules
func init() {
	id := make(matchMap, len(ID_RULES))
	pass := make(matchMap, len(PASS_RULES))
	for expr, present := range ID_RULES {
		id[expr] = matchRule{regexp.MustCompile(expr), present}
	}
	for expr, present := range PASS_RULES {
		pass[expr] = matchRule{regexp.MustCompile(expr), present}
	}
	matchers = struct {
		ID       matchMap
		Password matchMap
	}{
		ID:       id,
		Password: pass,
	}
}

func passStringReqs(s string, min int, max int, matches matchMap) bool {
	if len(s) < min || len(s) > max {
		return false
	}
	b := []byte(s)
	for _, rule := range matches {
		if rule.check.Match(b) != rule.present {
			return false
		}
	}
	return true
}
func isValidID(s string) bool {
	return passStringReqs(s, ID_MIN_LENGTH, ID_MAX_LENGTH, matchers.ID)
}
func isValidPass(s string) bool {
	return passStringReqs(s, PASS_MIN_LENGTH, PASS_MAX_LENGTH, matchers.Password)
}

func errWrite(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Write(newErrResponse(status, err.Error()))
}

func validateLoginBody(w http.ResponseWriter, r *http.Request, body *LoginBody) bool {
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		errWrite(w, http.StatusBadRequest, errors.New("cannot parse body"))
		return false
	}

	// field checks
	if !isValidID(body.ID) {
		errWrite(w, http.StatusUnprocessableEntity, ErrInvalidID)
		return false
	} else if !isValidPass(body.Password) {
		errWrite(w, http.StatusUnprocessableEntity, ErrInvalidPassword)
		return false
	}
	return true
}

func getUnverifiedTokenFromHeader(r *http.Header) []byte {
	auth := r.Get("Authorization")
	if auth == "" {
		return nil
	}
	matcher := regexp.MustCompile(`^Bearer ([a-zA-Z0-9_.\-]+$)`) //simple base64 char capture
	match := matcher.FindStringSubmatch(auth)
	if match == nil {
		return nil
	}
	return []byte(match[1]) // 0 is whole, not capture
}
func requestTokenReauthentication(w http.ResponseWriter) {
	w.Header().Add(
		"WWW-Authenticate",
		fmt.Sprintf(`Bearer error="invalid_token", error_description="%v"`, ErrInvalidToken),
	)
	errWrite(w, http.StatusUnauthorized, ErrInvalidToken)
}
func blockTokenUnauthorized(w http.ResponseWriter) {
	errWrite(w, http.StatusUnauthorized, ErrNoToken)
}

func ValidateLogin(w http.ResponseWriter, r *http.Request) {
	existingToken := getUnverifiedTokenFromHeader(&r.Header)
	if existingToken != nil && !ValidateToken(existingToken) {
		requestTokenReauthentication(w)
		return
	}

	var body LoginBody
	if !validateLoginBody(w, r, &body) {
		return
	}

	//try login
	data, err := getUserFromLogin(&body)
	switch err {
	case sql.ErrNoRows:
		errWrite(w, http.StatusUnauthorized, ErrInvalidLogin)
		return
	case nil:
	default:
		errWrite(w, http.StatusInternalServerError, err)
		return
	}

	//gen session token
	var token *jwt.Token
	if existingToken != nil {
		token, err = jwt.ParseNoVerify(existingToken) // it already passed ValidateToken
	} else {
		token, err = newTokenForUser(data)
	}
	if err != nil {
		errWrite(w, http.StatusInternalServerError, err)
		return
	}
	//
	payload, err := json.Marshal(&LoginResponse{
		Token:  token.String(),
		Expiry: int64(data.Expiry.Seconds()),
		User: UserData{
			ID: data.ID,
		},
	})
	if err != nil {
		errWrite(w, http.StatusInternalServerError, err)
		return
	}
	// success
	w.Write(payload)
}

func MiddlewareTokenValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		existingToken := getUnverifiedTokenFromHeader(&r.Header)
		if existingToken == nil {
			blockTokenUnauthorized(w)
			return
		} else if !ValidateToken(existingToken) {
			requestTokenReauthentication(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
