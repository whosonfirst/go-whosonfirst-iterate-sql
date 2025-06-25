package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"log/slog"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/whosonfirst/go-ioutil"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3/filters"
)

func init() {
	ctx := context.Background()
	iterate.RegisterIterator(ctx, "sql", NewSQLIterator)
}

// SQLIterator implements the `Iterator` interface for crawling records in a SQL database (specifically `database/sql` database with a 'geojson' table produced by `whosonfirst/go-whosonfirst-sqlite-features` and `whosonfirst/go-whosonfirst-sqlite-features-index`).
type SQLIterator struct {
	iterate.Iterator
	engine string
	// filters is a `whosonfirst/go-whosonfirst-iterate/v32/filters.Filters` instance used to include or exclude specific records from being crawled.
	filters filters.Filters
	// throttle is a channel used to control the maximum number of database rows that will be processed simultaneously.
	throttle chan bool
	// seen is the count of documents that have been processed.
	seen int64
	// iterating is a boolean value indicating whether records are still being iterated.
	iterating *atomic.Bool
}

// NewGitIterator() returns a new `GitIterator` instance configured by 'uri' in the form of:
//
//	sql://{ENGINE}?{PARAMETERS}
//
// {PARAMETERS} may be:
// * `?include=` Zero or more `aaronland/go-json-query` query strings containing rules that must match for a document to be considered for further processing.
// * `?exclude=` Zero or more `aaronland/go-json-query`	query strings containing rules that if matched will prevent a document from being considered for further processing.
// * `?include_mode=` A valid `aaronland/go-json-query` query mode string for testing inclusion rules.
// * `?exclude_mode=` A valid `aaronland/go-json-query` query mode string for testing exclusion rules.
// * `?processes=` An optional number assigning the maximum number of database rows that will be processed simultaneously. (Default is defined by `runtime.NumCPU()`.)
func NewSQLIterator(ctx context.Context, uri string) (iterate.Iterator, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	q := u.Query()

	max_procs := runtime.NumCPU()

	if q.Get("processes") != "" {

		procs, err := strconv.ParseInt(q.Get("processes"), 10, 64)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse 'processes' parameter, %w", err)
		}

		max_procs = int(procs)
	}

	throttle_ch := make(chan bool, max_procs)

	for i := 0; i < max_procs; i++ {
		throttle_ch <- true
	}

	f, err := filters.NewQueryFiltersFromQuery(ctx, q)

	if err != nil {
		return nil, fmt.Errorf("Failed to create query filters, %w", err)
	}

	it := &SQLIterator{
		engine:    u.Host,
		filters:   f,
		throttle:  throttle_ch,
		seen:      int64(0),
		iterating: new(atomic.Bool),
	}

	return it, nil
}

// Iterate will return an `iter.Seq2[*Record, error]` for each record encountered in 'uris'.
func (it *SQLIterator) Iterate(ctx context.Context, uris ...string) iter.Seq2[*iterate.Record, error] {

	return func(yield func(rec *iterate.Record, err error) bool) {

		it.iterating.Swap(true)
		defer it.iterating.Swap(false)

		sql_ctx, sql_cancel := context.WithCancel(ctx)
		defer sql_cancel()

		for _, uri := range uris {

			logger := slog.Default()
			logger = logger.With("uri", uri)

			conn, err := sql.Open(it.engine, uri)

			if err != nil {
				yield(nil, err)
				return
			}

			defer conn.Close()

			rows, err := conn.QueryContext(ctx, "SELECT id, body FROM geojson")

			if err != nil {
				if !yield(nil, fmt.Errorf("Failed to query 'geojson' table with '%s', %w", uri, err)) {
					return
				}

				continue
			}
			error_ch := make(chan error)
			rec_ch := make(chan *iterate.Record)

			wg := new(sync.WaitGroup)

			for rows.Next() {

				<-it.throttle

				var wofid int64
				var body string

				err := rows.Scan(&wofid, &body)

				if err != nil {
					if !yield(nil, fmt.Errorf("Failed to scan row with '%s', %w", uri, err)) {
						return
					}

					continue
				}

				wg.Add(1)

				go func(ctx context.Context, wofid int64, body string) {

					defer func() {
						it.throttle <- true
						wg.Done()
					}()

					select {
					case <-ctx.Done():
						return
					default:
						// pass
					}

					// uri := fmt.Sprintf("sqlite://%s#geojson:%d", path, wofid)

					// see the way we're passing in STDIN and not uri as the path?
					// that because we call ctx, err := ContextForPath(path) in the
					// process() method and since uri won't be there nothing will
					// get indexed - it's not ideal it's just what it is today...
					// (20171213/thisisaaronland)

					sr := strings.NewReader(body)

					rsc, err := ioutil.NewReadSeekCloser(sr)

					if err != nil {
						rsc.Close()
						error_ch <- fmt.Errorf("Failed to create ReadSeekCloser for record '%d' with '%s', %w", wofid, uri, err)
						return
					}

					if it.filters != nil {

						ok, err := iterate.ApplyFilters(ctx, rsc, it.filters)

						if err != nil {
							rsc.Close()
							error_ch <- fmt.Errorf("Failed to apply query filters to record '%d' with '%s', %w", wofid, uri, err)
							return
						}

						if !ok {
							rsc.Close()
							return
						}
					}

					rec_ch <- iterate.NewRecord(iterate.STDIN, rsc)

				}(sql_ctx, wofid, body)

				select {
				case err := <-error_ch:

					if !yield(nil, err) {
						return
					}
				case rec := <-rec_ch:

					if !yield(rec, nil) {
						return
					}
				default:
					// pass
				}
			}

			wg.Wait()

			err = rows.Err()

			if err != nil {
				yield(nil, err)
			}

		}
	}
}

// Seen() returns the total number of records processed so far.
func (it *SQLIterator) Seen() int64 {
	return atomic.LoadInt64(&it.seen)
}

// IsIterating() returns a boolean value indicating whether 'it' is still processing documents.
func (it *SQLIterator) IsIterating() bool {
	return it.iterating.Load()
}
