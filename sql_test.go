package sqlite

import (
	"context"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/whosonfirst/go-whosonfirst-iterate/v2/iterator"
)

func TestSQLiteEmitter(t *testing.T) {

	ctx := context.Background()

	rel_path := "fixtures/sfomuseum-maps.db"
	abs_path, err := filepath.Abs(rel_path)

	if err != nil {
		t.Fatalf("Failed to derive absolute path for '%s', %v", rel_path, err)
	}

	uris := map[string]int32{
		"sql://sqlite3":              int32(37),
		"sql://sqlite3?processes=10": int32(37),
		"sql://sqlite3?include=properties.sfomuseum:uri=2019": int32(1),
		"sql://sqlite3?exclude=properties.sfomuseum:uri=2019": int32(36),
	}

	for iter_uri, expected_count := range uris {

		count := int32(0)

		iter_cb := func(ctx context.Context, path string, r io.ReadSeeker, args ...interface{}) error {
			atomic.AddInt32(&count, 1)
			return nil
		}

		iter, err := iterator.NewIterator(ctx, iter_uri, iter_cb)

		if err != nil {
			t.Fatalf("Failed to create new iterator, %v", err)
		}

		err = iter.IterateURIs(ctx, abs_path)

		if err != nil {
			t.Fatalf("Failed to iterate %s, %v", abs_path, err)
		}

		if count != expected_count {
			t.Fatalf("Unexpected count for '%s': %d (expected: %d)", iter_uri, count, expected_count)
		}
	}
}
