build:
  mkdir -p .stead-test/go-build .stead-test/go-mod
  GOCACHE={{invocation_directory()}}/.stead-test/go-build GOMODCACHE={{invocation_directory()}}/.stead-test/go-mod sh scripts/build.sh {{bin}}

clean:
  rm -rf bin dist coverage
