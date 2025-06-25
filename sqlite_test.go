//go:build sqlite3

package sql

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/whosonfirst/go-whosonfirst-iterate/v3"
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

		iter, err := iterate.NewIterator(ctx, iter_uri)

		if err != nil {
			t.Fatalf("Failed to create new iterator, %v", err)
		}

		for rec, err := range iter.Iterate(abs_path) {

			if err != nil {
				t.Fatalf("Failed to walk '%s', %v", abs_path, err)
				break
			}

			_, err = io.ReadAll(rec.Body)

			if err != nil {
				t.Fatalf("Failed to read body for %s, %v", rec.Path, err)
			}

			_, err = rec.Body.Seek(0, 0)

			if err != nil {
				t.Fatalf("Failed to rewind body for %s, %v", rec.Path, err)
			}

			_, err = io.ReadAll(rec.Body)

			if err != nil {
				t.Fatalf("Failed second read body for %s, %v", rec.Path, err)
			}

			count += 1
		}

		if count != expected_count {
			t.Fatalf("Unexpected count for '%s'. Expected %d but got %d", uri, expected, count)
		}
	}
}
