package general

import (
	"encoding/json"
	"errors"
	"net/http"
	apiTypes "poll-bot/root/api/types"
	"poll-bot/root/types"
)

var bcp *types.BotCommandPackage
var ErrBCPNoInit = errors.New("bcp not initialized")

func SetBCP(newBCP *types.BotCommandPackage) {
	bcp = newBCP
}
func GetBCP() (*types.BotCommandPackage, error) {
	if bcp == nil {
		return nil, ErrBCPNoInit
	}
	return bcp, nil
}
func NewErrResponse(code int, msg string) []byte {
	resp, _ := json.Marshal(&apiTypes.ErrResponse{
		Code:    code,
		Error:   http.StatusText(code),
		Message: msg,
	})
	return resp
}

func ErrWrite(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	w.Write(NewErrResponse(status, err.Error())) //nolint
}
