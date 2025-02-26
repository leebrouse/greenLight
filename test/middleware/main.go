package main

import "net/http"

func main() {

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router(),
	}

	if err := srv.ListenAndServe(); err != nil {
		panic("Warning can Listening and Server")
	}
}
