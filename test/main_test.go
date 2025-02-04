package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkMarshalIndent(b *testing.B) {
	w := httptest.NewRecorder()
	r := new(http.Request)
	for n := 0; n < b.N; n++ {
		healthcheckHandlerMarshalIndent(w, r)
	}
}

func BenchmarkMarshal(b *testing.B) {
	w := httptest.NewRecorder()
	r := new(http.Request)
	for n := 0; n < b.N; n++ {
		healthcheckHandlerMarshal(w, r)
	}
}
