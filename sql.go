package sql

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	sfom_sql "github.com/sfomuseum/go-database/sql"
	"github.com/whosonfirst/go-ioutil"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3/filters"
)

func init() {
	ctx := context.Background()
	err := iterate.RegisterIterator(ctx, "sql", NewSQLIterator)

	if err != nil {
		panic(err)
	}
}

// SQLIterator implements the `Iterator` interface for crawling records in `database/sql` databases with a "geojson" table as defined by the `whosonfirst/go-whosonfirst-database` package.
type SQLIterator struct {
	iterate.Iterator
	// The `database/sql` engine (driver) to use for database connections.
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
// Where {ENGINE} is a registered `database/sql` driver and {PARAMETERS} may be:
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

			db_q := url.Values{}
			db_q.Set("dsn", uri)

			db_uri := url.URL{}
			db_uri.Scheme = "sql"
			db_uri.Host = it.engine
			db_uri.RawQuery = db_q.Encode()

			conn, err := sfom_sql.OpenWithURI(ctx, db_uri.String())

			if err != nil {
				logger.Error("Failed to open database connection", "error", err)
				yield(nil, err)
				return
			}

			defer conn.Close()

			rows, err := conn.QueryContext(ctx, "SELECT id, body FROM geojson")

			if err != nil {

				logger.Error("Failed to query geojson table", "error", err)

				if !yield(nil, fmt.Errorf("Failed to query 'geojson' table with '%s', %w", uri, err)) {
					return
				}

				continue
			}

			error_ch := make(chan error)
			done_ch := make(chan bool)
			rec_ch := make(chan *iterate.Record)

			remaining := 0

			for rows.Next() {

				var wofid int64
				var body string

				err := rows.Scan(&wofid, &body)

				if err != nil {
					logger.Error("Failed to scan row", "error", err)

					if !yield(nil, fmt.Errorf("Failed to scan row with '%s', %w", uri, err)) {
						return
					}

					continue
				}

				atomic.AddInt64(&it.seen, 1)
				remaining += 1

				go func(ctx context.Context, wofid int64, body string) {

					logger := slog.Default()
					logger = logger.With("uri", uri)
					logger = logger.With("id", wofid)

					defer func() {
						done_ch <- true
					}()

					select {
					case <-ctx.Done():
						return
					default:
						// pass
					}

					<-it.throttle

					defer func() {
						it.throttle <- true
					}()

					// uri := fmt.Sprintf("sqlite://%s#geojson:%d", path, wofid)

					// see the way we're passing in STDIN and not uri as the path?
					// that because we call ctx, err := ContextForPath(path) in the
					// process() method and since uri won't be there nothing will
					// get indexed - it's not ideal it's just what it is today...
					// (20171213/thisisaaronland)

					sr := strings.NewReader(body)

					rsc, err := ioutil.NewReadSeekCloser(sr)

					if err != nil {
						logger.Error("Failed to create ReadSeekCloser", "error", err)
						rsc.Close()
						error_ch <- fmt.Errorf("Failed to create ReadSeekCloser for record '%d' with '%s', %w", wofid, uri, err)
						return
					}

					if it.filters != nil {

						ok, err := iterate.ApplyFilters(ctx, rsc, it.filters)

						if err != nil {
							logger.Error("Failed to apply filters", "error", err)
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
			}

			err = rows.Err()

			if err != nil {
				logger.Error("Failed to iterate through rows", "error", err)
				yield(nil, err)
				return
			}

			for remaining > 0 {
				select {
				case <-done_ch:
					remaining -= 1
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

// Close performs any implementation specific tasks before terminating the iterator.
func (it *SQLIterator) Close() error {
	return nil
}
