.PHONY: all format lint generate test verify run

all: verify

format:
	buf format -w "proto"
	gofmt -w "cmd" "user"

lint:
	buf lint

generate:
	buf generate

test:
	go test -race "./..."

verify: lint generate test
	buf format --diff --exit-code "proto"

run:
	go run "./cmd/server"
