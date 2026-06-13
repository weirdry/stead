build:
  mkdir -p .stead-test/go-build
  GOCACHE={{invocation_directory()}}/.stead-test/go-build go build -o {{bin}} ./cmd/stead

clean:
  rm -rf bin dist coverage
