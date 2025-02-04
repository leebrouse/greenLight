package data

import (
	"time"
)

// {
// 	"id": 123,
// 	"title": "Casablanca",
// 	"runtime": 102,
// 	"genres": [
// 	"drama",
// 	"romance",
// 	"war"
// 	],
// 	"version": 1
// 	}

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
}
