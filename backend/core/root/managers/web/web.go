package web

import (
	"errors"
	"fmt"
	"poll-bot/root/api"
	api_general "poll-bot/root/api/general"
	api_login "poll-bot/root/api/login"
	"poll-bot/root/types"
	"sync/atomic"
	"time"
)

//singleton web metamanager

type WebManager struct{}

var manager atomic.Pointer[WebManager]

func New(port int, urlRootPath string, dbPath string, signingKey []byte, secureCookies bool) (man *WebManager, err error) {
	if manager.Load() != nil {
		return nil, errors.New("web manager already started")
	}

	man = &WebManager{}
	if !manager.CompareAndSwap(nil, man) {
		return nil, errors.New("web manager contested")
	}

	if err := api.Start(port, urlRootPath, dbPath, signingKey, secureCookies); err != nil {
		manager.CompareAndSwap(man, nil)
		return nil, fmt.Errorf("couldn't start webserver: %v", err)
	}
	return man, nil
}

// must be *BotCommandPackage
func GrantDiscordSessionInformation(req *types.BotCommandPackage) {
	api_general.SetBCP(req)
}
func (man *WebManager) ModifyUser(id string, password string, expiry time.Duration) error {
	return api_login.ModifyUser(id, password, expiry)
}
func (man *WebManager) DropUser(id string) error {
	return api_login.DropUser(id)
}
