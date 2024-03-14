#! /bin/bash

# Builds, tests and lints dendrite, and should be run before pushing commits

set -eu

echo "Generate go capnp code"
path="./**/*.capnp"
go_stdlib=$(getRealPath "./go-capnp/std")
capnp compile -I $go_stdlib --verbose -ogo $path

# Check that all the packages can build.
# When `go build` is given multiple packages it won't output anything, and just
# checks that everything builds.
echo "Checking that it builds..."
go build ./cmd/...

./build/scripts/find-lint.sh

echo "Testing..."
go test --race -v ./...
