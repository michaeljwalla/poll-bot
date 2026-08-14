package general

import (
	"encoding/json"
	"net/http"
	apiTypes "poll-bot/root/api/types"
	"poll-bot/root/types"
)

var bcp *types.BotCommandPackage

func SetBCP(newBCP *types.BotCommandPackage) {
	bcp = newBCP
}
func NewErrResponse(code int, msg string) []byte {
	resp, _ := json.Marshal(&apiTypes.ErrResponse{
		Code:    code,
		Error:   http.StatusText(code),
		Message: msg,
	})
	return resp
}
