package main

// Remember: support for individual database/sql engines is enabled through the use of build tags
// See sqlite3.go for an example

import (
	"context"
	"log"

	_ "github.com/whosonfirst/go-whosonfirst-iterate-sql/v3"
	"github.com/whosonfirst/go-whosonfirst-iterate/v3/app/emit"
)

func main() {

	ctx := context.Background()
	err := emit.Run(ctx)

	if err != nil {
		log.Fatalf("Failed to emit records, %v", err)
	}
}
