# Tigris NoQL generation tool

This tool looks up for the Tigris API calls in the source code and
generates query filters and update mutations for the filter and update
functions passed as parameters to the Tigris APIs.

# Install

```shell
go install github.com/tigrisdata/tigrisgen@latest
```

# Setup

Include the following line in one of the files in the package
which contains Tigris API calls:

```shell
//go:generate tigrisgen
```

Now, when building you project, before calling `go build` you need to run `go generate ./...` to
generate query filters and update mutations.

# License

This software is licensed under the [Apache 2.0](LICENSE).
