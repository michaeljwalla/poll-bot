package types

import "time"

type LoginBody struct {
	ID       string `json:"id"`
	Password string `json:"password"`

	// Hash []byte `json:"-"`
}

type ServerUserData struct {
	ID           string
	PasswordHash []byte

	//the current user's max session length
	Expiry time.Duration
}
type UserData struct {
	ID string `json:"id"`
}
type LoginResponse struct {
	Token  string   `json:"token"`
	Expiry int64    `json:"expires_in"`
	User   UserData `json:"user"`
}
type ErrResponse struct {
	Code    int    `json:"code"`
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
