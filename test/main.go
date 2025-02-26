package main

import (
	"flag"
	"log"
	"net/url"
	"strings"
)

var (
	urls []*url.URL
)

func main() {
	flag.Func("urls", "List all of the urls in the string", func(s string) error {

		for _, u := range strings.Fields(s) {
			if url, err := url.Parse(u); err != nil {
				return err
			} else {
				urls = append(urls, url)
			}
		}

		return nil
	})

	flag.Parse()

	for _, u := range urls {
		log.Println(u)
	}

}
