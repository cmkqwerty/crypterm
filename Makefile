build:
	@go build -o bin/crypterm

run: build
	@./bin/crypterm

.PHONY: build run
