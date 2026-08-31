package login

import (
	"fmt"
	"net/http"
	"poll-bot/root/api/general"
	"strconv"
	"sync"
	"time"
)

// A credential endpoint is the one door an unauthenticated caller can knock on
// as often as it likes, so failures are counted per source address and the
// address is shut out once it has spent its attempts.
const MAX_FAILED_LOGINS = 3
const LOGIN_BAN_DURATION = 5 * time.Minute

// Failures decay: an address that stops guessing for a whole ban's worth of
// time starts over, so a typo today does not count against a login next week.
const FAILURE_WINDOW = LOGIN_BAN_DURATION

var ErrLoginBanned = fmt.Errorf(
	"you've been banned from logging in for %d minutes",
	int(LOGIN_BAN_DURATION.Minutes()),
)

type loginAttempts struct {
	failures    int
	lastFailure time.Time
	bannedUntil time.Time
}

var attempts = struct {
	sync.Mutex
	byIP      map[string]*loginAttempts
	lastSweep time.Time
}{byIP: make(map[string]*loginAttempts)}

// caller must hold the lock
func sweepAttempts(now time.Time) {
	if now.Sub(attempts.lastSweep) < FAILURE_WINDOW {
		return
	}
	attempts.lastSweep = now
	for ip, entry := range attempts.byIP {
		if now.After(entry.bannedUntil) && now.Sub(entry.lastFailure) >= FAILURE_WINDOW {
			delete(attempts.byIP, ip)
		}
	}
}

// loginBannedUntil reports when the address may try again, if it is banned now.
func loginBannedUntil(ip string) (time.Time, bool) {
	now := time.Now()
	attempts.Lock()
	defer attempts.Unlock()
	sweepAttempts(now)

	entry := attempts.byIP[ip]
	if entry == nil || !now.Before(entry.bannedUntil) {
		return time.Time{}, false
	}
	return entry.bannedUntil, true
}

func recordLoginFailure(ip string) {
	now := time.Now()
	attempts.Lock()
	defer attempts.Unlock()

	entry := attempts.byIP[ip]
	if entry == nil {
		entry = &loginAttempts{}
		attempts.byIP[ip] = entry
	}
	if now.Sub(entry.lastFailure) >= FAILURE_WINDOW {
		entry.failures = 0
	}
	entry.failures++
	entry.lastFailure = now
	if entry.failures >= MAX_FAILED_LOGINS {
		entry.bannedUntil = now.Add(LOGIN_BAN_DURATION)
		entry.failures = 0
	}
}

func clearLoginFailures(ip string) {
	attempts.Lock()
	defer attempts.Unlock()
	delete(attempts.byIP, ip)
}

func blockLoginBanned(w http.ResponseWriter, until time.Time) {
	retry := int(time.Until(until).Seconds())
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	general.ErrWrite(w, http.StatusTooManyRequests, ErrLoginBanned)
}

// statusRecorder remembers what the wrapped handler answered, which is how the
// middleware tells a rejected credential from an accepted one without the
// handlers having to report back.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}
func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// A failed attempt is one the caller could have avoided by knowing the
// credentials: rejected pairs (401) and pairs that cannot belong to any
// account (422). A malformed body (400) is a broken client, not a guess.
func isFailedLogin(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusUnprocessableEntity
}

// MiddlewareLoginLimiter bans an address from the credential endpoints once it
// has burned MAX_FAILED_LOGINS attempts.
func MiddlewareLoginLimiter(next http.Handler) http.Handler {
	return loginLimiter(next, true)
}

// MiddlewareLoginBanCheck turns a ban away without counting anything against
// it. It belongs on the /login routes that carry a token rather than
// credentials: a stale session there is not a guess at a password, so it must
// not spend an attempt, but a banned address still has no business at any
// login door until its five minutes are up.
func MiddlewareLoginBanCheck(next http.Handler) http.Handler {
	return loginLimiter(next, false)
}

func loginLimiter(next http.Handler, count bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := requestIP(r)
		if until, banned := loginBannedUntil(ip); banned {
			blockLoginBanned(w, until)
			return
		}
		if !count {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)

		if isFailedLogin(recorder.status) {
			recordLoginFailure(ip)
		} else if recorder.status < http.StatusBadRequest {
			clearLoginFailures(ip)
		}
	})
}
