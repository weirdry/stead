fmt:
  gofmt -w ./cmd ./internal

test:
  mkdir -p .stead-test/go-build
  GOCACHE={{invocation_directory()}}/.stead-test/go-build go test ./...

check: fmt test
