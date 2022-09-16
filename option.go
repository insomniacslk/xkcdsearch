package xkcdsearch

import (
	"log"

	"golang.org/x/time/rate"
)

type Option func(*XKCDSearch)

func WithCacheDir(cachedir string) Option {
	return func(x *XKCDSearch) {
		x.cachedir = cachedir
	}
}

func WithRateLimit(r rate.Limit) Option {
	return func(x *XKCDSearch) {
		x.ratelimiter = rate.NewLimiter(r, 1)
	}
}

func WithLogger(l *log.Logger) Option {
	return func(x *XKCDSearch) {
		x.log = l
	}
}
