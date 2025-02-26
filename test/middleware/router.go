package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func router() http.Handler {
	router := httprouter.New()

	return middleware1(middleware2(middleware3(router)))
}
