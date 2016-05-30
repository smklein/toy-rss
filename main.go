package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
)

var flagURL string

// Init is a special function, which is called before main.
func init() {
	const (
		defaultURL = "https://www.reddit.com/.rss"
		usage      = "URL to be accessed"
	)
	flag.StringVar(&flagURL, "URL", defaultURL, usage)
	flag.StringVar(&flagURL, "u", defaultURL, usage+" (shorthand)")
}

func redirectPolicyFunc(req *http.Request, via []*http.Request) error {
	// TODO(smklein): Be a little smoother handling redirects in the future.
	log.Println("Request to redirect to: ", req)
	return errors.New("UNIMPLEMENTED: At the moment, we do not handle redirects")
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Parse()
	fmt.Println(">>> BROWSER START")
	fmt.Println("URL: " + flagURL)

	client := &http.Client{
		CheckRedirect: redirectPolicyFunc,
	}

	req, err := http.NewRequest("GET", flagURL, nil)
	check(err)

	req.Header.Set("user-Agent", "smklein's Golang RSS Reader")

	resp, err := client.Do(req)
	check(err)

	log.Println(resp)

	resp.Body.Close()
}
