package main

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/exp/slices"
	"log"
	"net/http"
)

func createVerifier(config *Configuration) *oidc.IDTokenVerifier {
	ctx := context.TODO()
	certsURL := fmt.Sprintf("%s/cdn-cgi/access/certs", config.Cloudflare.TeamDomain)

	oidcConfig := &oidc.Config{
		ClientID: config.Cloudflare.AUD,
	}
	keySet := oidc.NewRemoteKeySet(ctx, certsURL)
	verifier := oidc.NewVerifier(config.Cloudflare.TeamDomain, keySet, oidcConfig)

	return verifier
}

type AuthenticatedRequest struct {
	http.Request
	Username string
}

type AuthenticatedHandlerFunc func(http.ResponseWriter, *AuthenticatedRequest)

type Page struct {
	config   *Configuration
	verifier *oidc.IDTokenVerifier
}

func createPage(config *Configuration) *Page {
	verifier := createVerifier(config)
	page := &Page{config: config, verifier: verifier}
	return page
}

func (p *Page) wrap(toWrap AuthenticatedHandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		headers := r.Header

		// Make sure that the incoming request has our token header
		//  Could also look in the cookies for CF_AUTHORIZATION
		accessJWT := headers.Get("Cf-Access-Jwt-Assertion")
		if accessJWT == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("No token on the request"))
			return
		}

		// Verify the access token
		ctx := r.Context()
		idToken, err := p.verifier.Verify(ctx, accessJWT)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(fmt.Sprintf("Invalid token: %s", err.Error())))
			return
		}
		// Extract custom claims
		var claims struct {
			Email string `json:"email"`
		}
		if err := idToken.Claims(&claims); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(fmt.Sprintf("Token does not contain email: %s", err.Error())))
			return
		}

		if !slices.Contains(p.config.Users, claims.Email) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(fmt.Sprintf("User is not authorized: %s", claims.Email)))
			return
		}

		ar := &AuthenticatedRequest{Request: *r, Username: claims.Email}

		log.Printf("Request: %s %s %s %s",
			ar.Username, ar.Request.RemoteAddr, ar.Request.Method, ar.Request.URL)

		toWrap(w, ar)
	}
	return http.HandlerFunc(fn)
}
