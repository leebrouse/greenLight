package main

import (
	"fmt"
	"net/http"
)

func middleware1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("middleware1 start")
		next.ServeHTTP(w, r)
		fmt.Println("middleware1 end")
	})

}

func middleware2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("middleware2 start")
		next.ServeHTTP(w, r)
		fmt.Println("middleware2 end")
	})
}

func middleware3(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("final middleware")
	})

}
