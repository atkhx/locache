.PHONY: test
test:
	go clean -testcache
	go test -race  ./...

.PHONY: bench
bench:
	go test -bench  ./...

lint:
	golangci-lint run
