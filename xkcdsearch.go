package xkcdsearch

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/kirsle/configdir"
	"github.com/nishanths/go-xkcd"
	"golang.org/x/time/rate"
)

var (
	xkcdsearcher = New()
)

var (
	DefaultRateInterval = 100 * time.Millisecond
)

func Search(terms string) (string, error) {
	return xkcdsearcher.Search(terms)
}

func Update() error {
	return xkcdsearcher.Update()
}

func New(opts ...Option) *XKCDSearch {
	x := XKCDSearch{
		ratelimiter: rate.NewLimiter(rate.Every(DefaultRateInterval), 1),
		log:         log.New(os.Stderr, "", log.Ldate|log.Ltime),
	}
	for _, opt := range opts {
		opt(&x)
	}
	if x.cachedir == "" {
		x.cachedir = configdir.LocalConfig("xkcdsearch")
	}
	return &x
}

type XKCDSearch struct {
	cachedir    string
	index       bleve.Index
	ratelimiter *rate.Limiter
	log         *log.Logger
}

func (x *XKCDSearch) Update() error {
	var toFetch []int
	client := xkcd.NewClient()
	ctx := context.Background()
	latest, err := client.Latest(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest comic metadata: %w", err)
	}

	// get the index
	existing := make(map[int]bool, 0)
	x.log.Printf("Opening index at '%s'", x.cachedir)
	index, err := bleve.Open(x.cachedir)
	if err != nil {
		if err == bleve.ErrorIndexPathDoesNotExist {
			x.log.Printf("Index does not exist, creating a new one")
			mapping := bleve.NewIndexMapping()
			index, err = bleve.New(x.cachedir, mapping)
			if err != nil {
				return fmt.Errorf("failed to create index: %w", err)
			}
			for num := 1; num <= latest.Number; num++ {
				toFetch = append(toFetch, num)
			}
		} else {
			return fmt.Errorf("failed to open index: %w", err)
		}
	} else {
		x.log.Printf("Index exists, checking if it needs to be updated")
		// check if the index needs to be updated
		query := bleve.NewMatchAllQuery()
		// Maximum number of results to fetch. We want all of them but there is
		// no way to tell Bleve to use no limit, so I use the highest number we
		// can.
		size := math.MaxInt
		// don't skip any document
		skip := 0
		// don't ask to explain the score
		explain := false
		search := bleve.NewSearchRequestOptions(query, size, skip, explain)
		search.Fields = []string{"Number"}
		results, err := index.Search(search)
		if err != nil {
			return fmt.Errorf("failed to get all comics: %w", err)
		}
		// build a map of the indexes of all the comics we have already
		x.log.Printf("Hits: %d", len(results.Hits))
		for _, item := range results.Hits {
			for name, value := range item.Fields {
				if name == "Number" {
					existing[int(value.(float64))] = true
				}
			}
		}
		// then build the inverse list of comics we want to fetch
		for num := 1; num <= latest.Number; num++ {
			if _, ok := existing[num]; !ok {
				toFetch = append(toFetch, num)
			}
		}
	}
	x.log.Printf("There are %d comics indexed out of %d, will fetch %d comics", len(existing), latest.Number, len(toFetch))
	sort.Ints(toFetch)

	// finally, fetch and index
	var comics []xkcd.Comic
	var wg sync.WaitGroup
	comicChan := make(chan *xkcd.Comic, 1)
	go func(ch <-chan *xkcd.Comic) {
		count := 0
		for comic := range ch {
			comics = append(comics, *comic)
			count++
			if count%100 == 0 {
				x.log.Printf("Fetched %d comics..\n", count)
			}
		}
		x.log.Printf("Fetched %d comics..\n", count)
	}(comicChan)
	for _, num := range toFetch {
		if num == 404 {
			continue
		}
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			if err := x.ratelimiter.Wait(ctx); err != nil {
				x.log.Printf("Rate limiter Wait failed: %v", err)
				return
			}
			comic, err := client.Get(ctx, num)
			if err != nil {
				x.log.Printf("Failed to get metadata for xkcd.com/%d: %v", num, err)
				return
			}
			comicChan <- &comic
		}(num)
	}
	wg.Wait()

	// index all the comics in a batch
	batch := index.NewBatch()
	for _, comic := range comics {
		x.log.Printf("Indexing comic ID %d \"%s\"\n", comic.Number, comic.Title)
		if err := batch.Index(strconv.FormatInt(int64(comic.Number), 10), comic); err != nil {
			x.log.Printf("Failed to index comic %d: %v", comic.Number, err)
		}
	}
	if err := index.Batch(batch); err != nil {
		return fmt.Errorf("batching failed: %w", err)
	}

	x.index = index
	return nil
}

func (x *XKCDSearch) Search(terms string) (string, error) {
	if x.index == nil {
		if err := x.Update(); err != nil {
			return "", err
		}
	}
	query := bleve.NewMatchQuery(terms)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"ImageURL", "Alt", "Number", "Title"}
	results, err := x.index.Search(search)
	if err != nil {
		return "", err
	}
	res := *results
	if len(res.Hits) == 0 {
		return "not found", nil
	}
	for name, value := range res.Hits[0].Fields {
		if name == "ImageURL" {
			return fmt.Sprintf("%v", value), nil
		}
	}
	return "", fmt.Errorf("no image URL found for comic %+v", res)
}
