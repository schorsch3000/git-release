.PHONY: build test install
build:
	go build -o git-release .
test:
	go vet ./... && go test ./...
install:
	go install .
