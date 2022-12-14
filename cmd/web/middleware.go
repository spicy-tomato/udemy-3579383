package main

import (
	"github.com/justinas/nosurf"
	"learn-golang/internal/helpers"
	"net/http"
)

// NoSurf add CSRF protection to all POST request
func NoSurf(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(
		http.Cookie{
			HttpOnly: true,
			Path:     "/",
			Secure:   app.InProduction,
			SameSite: http.SameSiteLaxMode,
		},
	)
	return csrfHandler
}

// SessionLoad loads and saves the session on every request
func SessionLoad(next http.Handler) http.Handler {
	return session.LoadAndSave(next)
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if !helpers.IsAuthenticated(r) {
				session.Put(r.Context(), "error", "Login first!")
				http.Redirect(w, r, "/user/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		},
	)
}
