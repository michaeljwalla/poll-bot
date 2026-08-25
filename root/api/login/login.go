package login

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"poll-bot/root/api/general"
	"poll-bot/root/api/types"
	"regexp"
	"strings"
)

var ErrBadBody = errors.New("cannot parse body")

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

func validateLoginBody(w http.ResponseWriter, r *http.Request, body *LoginBody) bool {
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		general.ErrWrite(w, http.StatusBadRequest, ErrBadBody)
		return false
	}

	// field checks
	if !isValidID(body.ID) {
		general.ErrWrite(w, http.StatusUnprocessableEntity, ErrInvalidID)
		return false
	} else if !isValidPass(body.Password) {
		general.ErrWrite(w, http.StatusUnprocessableEntity, ErrInvalidPassword)
		return false
	}
	return true
}

// The session cookie is scoped to the path the app is actually mounted at, so
// hosting under a shared domain (my-site.com/pb) does not hand the token to
// every other app on that domain. Secure is off by default because plain-http
// localhost is the dev target; PROD turns it on.
var cookieOpts = struct {
	Path   string
	Secure bool
}{Path: "/"}

// SetCookieOptions must be called before the server starts serving.
func SetCookieOptions(rootPath string, secure bool) {
	path := strings.TrimSuffix(rootPath, "/")
	if path == "" {
		path = "/"
	}
	cookieOpts.Path = path
	cookieOpts.Secure = secure
}

func getUnverifiedTokenFromCookie(r *http.Request) ([]byte, error) {
	auth, err := r.Cookie("Authorization")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, err
	}

	matcher := regexp.MustCompile(`^([a-zA-Z0-9_.\-]+$)`) //simple base64 char capture
	match := matcher.FindStringSubmatch(auth.Value)
	if match == nil {
		return nil, nil
	}
	return []byte(match[1]), nil // 0 is whole, not capture
}
func requestTokenReauthentication(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "Authorization",
		Value:    "",
		HttpOnly: true,
		Secure:   cookieOpts.Secure,
		SameSite: http.SameSiteLaxMode,
		Path:     cookieOpts.Path,
		MaxAge:   -1,
	})
	general.ErrWrite(w, http.StatusUnauthorized, ErrInvalidToken)
}
func blockTokenUnauthorized(w http.ResponseWriter) {
	general.ErrWrite(w, http.StatusUnauthorized, ErrNoToken)
}

func checkLogin(w http.ResponseWriter, r *http.Request) (*ServerUserData, error) {
	var body LoginBody
	if !validateLoginBody(w, r, &body) {
		return nil, ErrBadBody
	}

	//try login
	data, err := getUserFromLogin(&body)
	switch err {
	case sql.ErrNoRows:
		general.ErrWrite(w, http.StatusUnauthorized, ErrInvalidLogin)
		return nil, ErrInvalidLogin
	case nil:
	default:
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return nil, err
	}
	return data, err
}
func ValidateLogin(w http.ResponseWriter, r *http.Request) {
	data, err := checkLogin(w, r)
	if err != nil {
		return
	}

	//gen session token
	token, err := newTokenForUser(data)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	//
	http.SetCookie(w, &http.Cookie{
		Name:     "Authorization",
		Value:    token.String(),
		HttpOnly: true,
		Secure:   cookieOpts.Secure,
		SameSite: http.SameSiteLaxMode,
		Path:     cookieOpts.Path,
		MaxAge:   int(data.Expiry.Seconds()),
	})
	w.WriteHeader(200)
}

type TokenJSON struct {
	Token string `json:"token"`
}

func GetLoginTokenJSON(w http.ResponseWriter, r *http.Request) {
	token, _ := getUnverifiedTokenFromCookie(r)
	if token == nil {
		data, err := checkLogin(w, r)
		if err != nil {
			return
		}

		//gen session token
		jwt, err := newTokenForUser(data)
		if err != nil {
			general.ErrWrite(w, http.StatusInternalServerError, err)
			return
		}
		token = jwt.Bytes()
	}

	data, err := json.Marshal(&TokenJSON{string(token)})
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
func MiddlewareTokenValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		existingToken, err := getUnverifiedTokenFromCookie(r)
		if err != nil {
			general.ErrWrite(w, http.StatusInternalServerError, err)
			return
		}
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

type CheckResponse struct {
	//unix milliseconds, so the browser can use it without unit juggling
	ExpiresAt int64 `json:"expiresAt"`
}

// CheckLogin sits behind MiddlewareTokenValidator, so reaching it at all means
// the cookie is valid; the body only carries how long that stays true.
func CheckLogin(w http.ResponseWriter, r *http.Request) {
	token, err := getUnverifiedTokenFromCookie(r)
	if err != nil || token == nil {
		general.ErrWrite(w, http.StatusUnauthorized, ErrNoToken)
		return
	}
	expiry, ok := TokenExpiry(token)
	if !ok {
		general.ErrWrite(w, http.StatusUnauthorized, ErrInvalidToken)
		return
	}
	body, err := json.Marshal(&CheckResponse{ExpiresAt: expiry.UnixMilli()})
	if err != nil {
		general.ErrWrite(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body) //nolint
}
