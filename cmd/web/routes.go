package main

import (
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
)

func (app *Config) Routes() http.Handler {
	mux := chi.NewRouter()

	//middleware
	mux.Use(middleware.Recoverer)

	// //define app routes
	mux.Use(app.SessionLoad)
	mux.Get("/", app.HomePage)
	mux.Get("/login", app.LoginPage)
	mux.Post("/login", app.PostLoginPage)
	mux.Get("/logout", app.Logout)
	mux.Get("/register", app.RegisterPage)
	mux.Post("/register", app.PostRegisterPage)
	mux.Get("/activate", app.ActivateAccount)
	mux.Mount("/members", app.AuthRouter())
	return mux
}

func (app *Config) AuthRouter() http.Handler {
	mux := chi.NewRouter()
	mux.Use(app.Auth)
	mux.Get("/plan", app.ChooseSubscription)
	mux.Get("/subscribe", app.SubscribeToPlan)
	return mux
}
