package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/leebrouse/greenLight/internal/data"
	"github.com/leebrouse/greenLight/internal/validator"
	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// ratelimited middle to prevent exceeded request from the users (avg limit = 2  max=4)
func (app *application) ratelimited(next http.Handler) http.Handler {

	type client struct {
		limiter  *rate.Limiter
		lastseen time.Time
	}

	// type和var 区别

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// delete leisure client to protect the limited resource
	go func() {
		for {
			time.Sleep(time.Minute)

			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastseen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if app.config.limiter.enable {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

			//lock
			mu.Lock()

			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
				}
			}

			clients[ip].lastseen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}

			//unlock
			mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//set response header
		w.Header().Add("Vary", "Authorization")

		//get value from the request header by the key Authorization
		authorizationHeader := r.Header.Get("Authorization")

		//if Authorization is empty, set a AnonymousUser then to the next handler
		if authorizationHeader == "" {
			r := app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		//split the string from the Authorization string array
		//eg:"Authorization: Bearer L3Q53W5K2YV4GXGFDK5XN25DQQ"---> ["Bearer","L3Q53W5K2YV4GXGFDK5XN25DQQ"]
		headerParts := strings.Split(authorizationHeader, " ")
		//check the string array
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidCredentialsResponse(w, r)
			return
		}
		//get token
		token := headerParts[1]

		//new Validatir
		v := validator.New()

		//check token format
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		//Get user by the given token
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			//if recordNotFound return invaildAuthorization error
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		//call contextSetUser
		r = app.contextSetUser(r, user)
		//next
		next.ServeHTTP(w, r)

	})
}
