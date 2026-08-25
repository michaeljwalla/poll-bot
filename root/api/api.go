package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"poll-bot/root/api/alias"
	"poll-bot/root/api/login"
	"poll-bot/root/api/polls"
	"poll-bot/root/api/web"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const API_VER = "v1"

func initAuthorizers(dbPath string, rootPath string, signingKey []byte, secureCookies bool) error {
	if len(signingKey) == 0 {
		return errors.New("no signing key provided")
	}
	if err := login.Start(dbPath, signingKey); err != nil {
		return err
	}
	login.SetCookieOptions(rootPath, secureCookies)
	return nil
}
func initRouter(port int, rootPath string) error {
	//chi http template
	r := chi.NewRouter()
	validator := login.MiddlewareTokenValidator
	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(60 * time.Second))

	// API
	r.Route(rootPath+"/api/"+API_VER, func(r chi.Router) {
		// login (no validator)
		r.Post("/login", login.ValidateLogin)
		r.Post("/login/json", login.GetLoginTokenJSON)
		r.With(validator).Get("/login/check", login.CheckLogin)

		// paginated polls getter
		r.With(validator).Get("/polls", polls.GetPage)
		r.With(validator).Put("/polls", polls.AddPolls)
		r.With(validator).Patch("/polls/activate", polls.SetActive)
		r.With(validator).Patch("/polls/title", polls.SetTitle)
		r.With(validator).Delete("/polls", polls.DropPolls)

		// aliases endpoint
		r.With(validator).Get("/aliases", alias.GetAliases)
		r.With(validator).Put("/aliases", alias.SetAliases)
		r.With(validator).Delete("/aliases", alias.DropAliases)
	})

	// FRONTEND
	// Registered last and as a wildcard, so it only sees what the API routes
	// above did not claim. Both patterns are needed: chi's "/*" does not match
	// the bare prefix that a link to the app's root produces.
	webHandler, err := web.Handler(rootPath)
	if err != nil {
		if !errors.Is(err, web.ErrNotBuilt) && !errors.Is(err, web.ErrBaseMismatch) {
			return err
		}
		//the bot is the product; an unusable frontend must not stop it booting.
		log.Println("web:", err)
		webHandler = web.Notice(err.Error())
	}
	r.Handle(rootPath+"/", webHandler)
	r.Handle(rootPath+"/*", webHandler)
	if rootPath != "" {
		//"/pb" with no trailing slash would otherwise miss both patterns
		r.Handle(rootPath, http.RedirectHandler(rootPath+"/", http.StatusMovedPermanently))
	}

	go http.ListenAndServe(fmt.Sprintf(":%d", port), r) //nolint
	return nil
}

// listening port, path to db, signing key
func Start(port int, urlRootPath string, dbPath string, signingKey []byte, secureCookies bool) error {
	if err := initAuthorizers(dbPath, urlRootPath, signingKey, secureCookies); err != nil {
		return err
	}
	if err := initRouter(port, urlRootPath); err != nil {
		return err
	}
	return nil
}
