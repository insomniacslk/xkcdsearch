package xkcdsearch

import (
	"context"
	"fmt"
	"strconv"

	"github.com/blevesearch/bleve/v2"
	"github.com/kirsle/configdir"
	"github.com/nishanths/go-xkcd"
)

var (
	defaultCacheDir = ""
	xkcdsearcher    = NewXKCDSearch(defaultCacheDir)
)

func Search(terms string) (string, error) {
	return xkcdsearcher.Search(terms)
}

func Update() error {
	return xkcdsearcher.Update()
}

func FetchAll() ([]xkcd.Comic, error) {
	return xkcdsearcher.FetchAll()
}

func NewXKCDSearch(cachedir string) *XKCDSearch {
	if cachedir == "" {
		cachedir = configdir.LocalConfig("xkcdsearch")
	}
	return &XKCDSearch{
		cachedir: cachedir,
	}
}

type XKCDSearch struct {
	cachedir string
	index    bleve.Index
}

func (x *XKCDSearch) FetchAll() ([]xkcd.Comic, error) {
	ctx := context.Background()
	client := xkcd.NewClient()
	latest, err := client.Latest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for latest comic: %w", err)
	}
	comics := make([]xkcd.Comic, 0, latest.Number)
	for idx := 1; idx <= latest.Number; idx++ {
		if idx == 404 {
			// well...
			continue
		}
		comic, err := client.Get(ctx, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to get metadata for xkcd.com/%d: %w", idx, err)
		}
		comics = append(comics, comic)
	}
	return comics, nil
}

func (x *XKCDSearch) Update() error {
	index, err := bleve.Open(x.cachedir)
	if err != nil && err == bleve.ErrorIndexPathDoesNotExist {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(x.cachedir, mapping)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
		comics, err := x.FetchAll()
		if err != nil {
			return fmt.Errorf("FetchAll failed: %w", err)
		}
		for idx, comic := range comics {
			// do indexing
			if err := index.Index(strconv.FormatInt(int64(comic.Number), 10), comic); err != nil {
				return fmt.Errorf("failed to index item %d: %w", idx, err)
			}
			fmt.Println(comic.Alt)
		}
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
