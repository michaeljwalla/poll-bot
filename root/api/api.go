package api

import (
	"errors"
	"fmt"
	"net/http"
	"poll-bot/root/api/login"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const API_VER = "v1"

func initAuthorizers(dbPath string, signingKey []byte) error {
	if len(signingKey) == 0 {
		return errors.New("no signing key provided")
	}
	if err := login.Start(dbPath, signingKey); err != nil {
		return err
	}
	return nil
}
func initRouter(port int, rootPath string) error {
	//chi http template
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIPFromRemoteAddr) // pick one ClientIPFrom* based on your infra, see below
	// r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(60 * time.Second))

	// USER
	r.Get(rootPath+"/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello " + API_VER))
	})

	// API
	r.Route(rootPath+"/api/"+API_VER, func(r chi.Router) {
		r.Post("/login", login.ValidateLogin)
	})

	go http.ListenAndServe(fmt.Sprintf(":%d", port), r) //always nonnil
	return nil
}

// listening port, path to db, signing key
func Start(port int, urlRootPath string, dbPath string, signingKey []byte) error {
	if err := initAuthorizers(dbPath, signingKey); err != nil {
		return err
	}
	if err := initRouter(port, urlRootPath); err != nil {
		return err
	}
	return nil
}
