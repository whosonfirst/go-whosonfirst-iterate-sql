# go-whosonfirst-iterate-sql

Go package implementing `whosonfirst/go-whosonfirst-iterate/v3.Iterator` functionality for `database/sql` compatiable databases.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/whosonfirst/go-whosonfirst-iterate-sqlite.svg)](https://pkg.go.dev/github.com/whosonfirst/go-whosonfirst-iterate-sql/v3)

## Building (database support)

This package uses Go language build tags to enable support for individual `database/sql` compatiable databases. Currently only the [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) package is available by default when the `sqlite3` tag is defined. Other databases can be added as needed (see [sqlite3.go](sqlite3.go) for an example).

The only requirement is a `geojson` table as defined by the [whosonfirst/go-whosonfirst-database](https://github.com/whosonfirst/go-whosonfirst-database/tree/main/sql/tables) package which can be produced using tools in the [whosonfirst/go-whosonfirst-database-sqlite](https://github.com/whosonfirst/go-whosonfirst-database-sqlite) package.

Database connections are opened using the [sfomuseum/go-database/sql.OpenWithURI](https://github.com/sfomuseum/go-database/blob/main/sql/database.go#L36) utility method which in turn will apply some [opinionated database PRAGAM](https://github.com/sfomuseum/go-database/blob/main/sql/sqlite.go#L10) for SQLite databases.

## Example

Version 3.x of this package introduce major, backward-incompatible changes from earlier releases. That said, migragting from version 2.x to 3.x should be relatively straightforward as a the _basic_ concepts are still the same but (hopefully) simplified. Where version 2.x relied on defining a custom callback for looping over records version 3.x use Go's [iter.Seq2](https://pkg.go.dev/iter) iterator construct to yield records as they are encountered.


```
import (
	"context"
	"flag"
	"log"

	_ "github.com/whosonfirst/go-whosonfirst-iterate-sql/v3"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3"
)

func main() {

     	var iterator_uri string

	flag.StringVar(&iterator_uri, "iterator-uri", "sql://sqlite3". "A registered whosonfirst/go-whosonfirst-iterate/v3.Iterator URI.")
	ctx := context.Background()
	
	iter, _:= iterate.NewIterator(ctx, iterator_uri)

	paths := flag.Args()
	
	for rec, _ := range iter.Iterate(ctx, paths...) {
		log.Printf("Indexing %s\n", rec.Path)
	}
}
```

_Error handling removed for the sake of brevity._

### Version 2.x (the old way)

This is how you would do the same thing using the older version 2.x code:

```
import (
       "context"
       "fmt"
       "io"

       _ "github.com/mattn/go-sqlite3"
       _ "github.com/whosonfirst/go-whosonfirst-iterate-sql/v2"
       
       "github.com/whosonfirst/go-whosonfirst-iterate/v2/iterator"
)

func main() {

	ctx := context.Background()
     
	iter_cb := func(ctx context.Context, path string, r io.ReadSeeker, args ...interface{}) error {
		fmt.Println(path)
		return nil
	}

	iter, _ := iterator.NewIterator(ctx, "sql://sqlite3", iter_cb)

	iter.IterateURIs(ctx, "whosonfirst.db")
}	
```

## Tools

```
$> make cli
go build -tags sqlite3 -mod readonly -ldflags="-s -w" -o bin/count cmd/count/main.go
go build -tags sqlite3 -mod readonly -ldflags="-s -w" -o bin/emit cmd/emit/main.go
```

### count

Count files in one or more whosonfirst/go-whosonfirst-iterate/v3.Iterator sources.

```
$> ./bin/count -h
Count files in one or more whosonfirst/go-whosonfirst-iterate/v3.Iterator sources.
Usage:
	 ./bin/count [options] uri(N) uri(N)
Valid options are:

  -iterator-uri string
    	A valid whosonfirst/go-whosonfirst-iterate/v3.Iterator URI. Supported iterator URI schemes are: cwd://,directory://,featurecollection://,file://,filelist://,geojsonl://,null://,repo://,sql:// (default "repo://")
  -verbose
    	Enable verbose (debug) logging.
```	

For example:

```
$> ./bin/count -iterator-uri 'sql://sqlite3' ./fixtures/sfomuseum-maps.db 
2025/06/25 06:18:51 INFO Counted records count=37 time=3.301824ms
``

### emit

Emit records in one or more whosonfirst/go-whosonfirst-iterate/v3.Iterator sources as structured data.

```
$> ./bin/emit -h
Emit records in one or more whosonfirst/go-whosonfirst-iterate/v3.Iterator sources as structured data.
Usage:
	 ./bin/emit [options] uri(N) uri(N)
Valid options are:

  -geojson
    	Emit features as a well-formed GeoJSON FeatureCollection record.
  -iterator-uri string
    	A valid whosonfirst/go-whosonfirst-iterate/v3.Iterator URI. Supported iterator URI schemes are: cwd://,directory://,featurecollection://,file://,filelist://,geojsonl://,null://,repo://,sql:// (default "repo://")
  -json
    	Emit features as a well-formed JSON array.
  -null
    	Publish features to /dev/null
  -stdout
    	Publish features to STDOUT. (default true)
  -verbose
    	Enable verbose (debug) logging.
```

For example:

```
$> ./bin/emit -geojson -iterator-uri 'sql://sqlite3' ./fixtures/sfomuseum-maps.db | jq -r '.features[]["properties"]["wof:name"]'
SFO (2018)
SFO (1947)
SFO (1980)
SFO (1949)
SFO (1956)
SFO (1997)
SFO (1989)
SFO (1985)
SFO (2017)
SFO (1943)
SFO (1960)
...and so on
```

## See also

* https://github.com/whosonfirst/go-whosonfirst-iterate
* https://pkg.go.dev/database/sql
* https://github.com/mattn/go-sqlite3
* https://github.com/whosonfirst/go-whosonfirst-database
* https://github.com/whosonfirst/go-whosonfirst-database-sqlite
* https://github.com/sfomuseum/go-database/