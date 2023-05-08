all:
	go build .

unit_test:
	go test ./...

test: unit_test

lint:
	golangci-lint run --fix
