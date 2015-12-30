package main

import (
	"github.com/abbot/go-http-auth"
	"log"
	"net/http"
)

type Page struct {
	config *Configuration
	auth   auth.AuthenticatorInterface
}

func createPage(config *Configuration) *Page {
	page := &Page{config: config}
	authenticator := auth.NewDigestAuthenticator("garagebot", page.secret)
	page.auth = authenticator
	return page
}

func (p *Page) secret(user, realm string) string {
	return p.config.Users[user]
}

func (p *Page) wrap(toWrap auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return p.auth.Wrap(func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		log.Printf("Request: %s %s %s %s",
			r.Username, r.Request.RemoteAddr, r.Request.Method, r.Request.URL)
		toWrap(w, r)
	})
}
