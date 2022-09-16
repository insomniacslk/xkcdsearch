package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/insomniacslk/xkcdsearch"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
)

var (
	flagCacheDir  = pflag.StringP("cachedir", "c", "", "Cache directory where the index is stored")
	flagRateLimit = pflag.DurationP("rate-limit", "l", xkcdsearch.DefaultRateInterval, "Rate limit for how fast to fetch XKCD comics during an update, expressed as time string (e.g. 10ms)")
)

func main() {
	pflag.Parse()
	if len(pflag.Args()) < 1 {
		log.Fatalf("No search term specified")
	}
	terms := strings.Join(pflag.Args(), " ")
	xkcd := xkcdsearch.New(
		xkcdsearch.WithCacheDir(*flagCacheDir),
		xkcdsearch.WithRateLimit(rate.Every(*flagRateLimit)),
	)
	comic, err := xkcd.Search(terms)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Title    : %s\n", comic.Title)
	fmt.Printf("Number   : %d\n", comic.Number)
	fmt.Printf("URL      : https://xkcd.com/%d\n", comic.Number)
	fmt.Printf("ImageURL : %s\n", comic.ImageURL)
	fmt.Printf("Alt      : %s\n", comic.Alt)
}
