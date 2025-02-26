package main

import (
	"errors"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
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
		//eg:"Authorization: Bearer L3Q53W5K2YV4GXGFDK5XN25DQQ"	---> ["Bearer","L3Q53W5K2YV4GXGFDK5XN25DQQ"]
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

/* Moive part middle */
// Create a new requireAuthenticatedUser() middleware to check that a user is not anonymous.
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//get user from the request
		user := app.contextGetUser(r)

		//if user is AnonymousUser :
		//send a 401 Unauthorized response(“you must be authenticated to access this resource”)
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		//next handler
		next.ServeHTTP(w, r)
	})
}

// Checks that a user is both authenticated and activated.
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//get user from the request context
		user := app.contextGetUser(r)

		//if user is not activated:
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		//next handler
		next.ServeHTTP(w, r)
	})

	return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the user from the request context.
		user := app.contextGetUser(r)
		// Get the slice of permissions for the user.
		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		// Check if the slice includes the required permission. If it doesn't, then
		// return a 403 Forbidden response.
		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}
		// Otherwise they have the required permission so we call the next handler in
		// the chain.
		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add the "Vary: Origin" header.
		w.Header().Add("Vary", "Origin")
		// Get the value of the request's Origin header.
		origin := r.Header.Get("Origin")

		if origin != "" && len(app.config.cors.trustedOrigins) != 0 {
			if slices.Contains(app.config.cors.trustedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)

				// Check if the request has the HTTP method OPTIONS and contains the
				// "Access-Control-Request-Method" header. If it does, then we treat
				// it as a preflight request.
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					// Set the necessary preflight response headers, as discussed
					// previously.
					w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					// Write the headers along with a 200 OK status and return from
					// the middleware with no further action.
					w.WriteHeader(http.StatusOK)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) metrics(next http.Handler) http.Handler {
	totalRequestReceived := expvar.NewInt("total_request_received")
	totalResponseSent := expvar.NewInt("total_resonse_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")

	totalResponseSentByStatus := expvar.NewMap("total_response_sent_by_status")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record the time that we started to process the request.
		start := time.Now()

		// Use the Add() method to increment the number of requests received by 1
		totalRequestReceived.Add(1)

		// Call the httpsnoop.CaptureMetrics() function, passing in the next handler in
		// the chain along with the existing http.ResponseWriter and http.Request. This
		// returns the metrics struct that we saw above.
		metrics := httpsnoop.CaptureMetrics(next, w, r)
		// On the way back up the middleware chain, increment the number of responses
		// sent by 1.
		totalResponseSent.Add(1)
		// Calculate the number of microseconds since we began to process the request,
		duration := time.Since(start).Microseconds()

		// then increment the total processing time by this amount.
		totalProcessingTimeMicroseconds.Add(duration)

		totalResponseSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
	})

}
