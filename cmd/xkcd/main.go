package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/insomniacslk/xkcdsearch"
	"github.com/spf13/pflag"
)

func main() {
	pflag.Parse()
	if len(pflag.Args()) < 1 {
		log.Fatalf("No search term specified")
	}
	terms := strings.Join(pflag.Args(), " ")
	link, err := xkcdsearch.Search(terms)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(link)
}
