GOMOD=$(shell test -f "go.work" && echo "readonly" || echo "vendor")
LDFLAGS=-s -w

TAGS=sqlite3

cli:
	go build -tags $(TAGS) -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/count cmd/count/main.go
	go build -tags $(TAGS) -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/emit cmd/emit/main.go

tests:
	go test -tags $(TAGS) -v ./...
