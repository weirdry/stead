fmt:
  gofmt -w ./cmd ./internal

test:
  mkdir -p .stead-test/go-build .stead-test/go-mod
  GOCACHE={{invocation_directory()}}/.stead-test/go-build GOMODCACHE={{invocation_directory()}}/.stead-test/go-mod go test ./...

check: fmt test
