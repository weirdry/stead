build:
  mkdir -p .stead-test/go-build .stead-test/go-mod
  GOCACHE={{invocation_directory()}}/.stead-test/go-build GOMODCACHE={{invocation_directory()}}/.stead-test/go-mod go build -o {{bin}} ./cmd/stead

clean:
  rm -rf bin dist coverage
