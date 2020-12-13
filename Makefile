PROGRAM=linnet

.PHONY: format
format:
	@find . -type f -name "*.go*" -print0 | xargs -0 gofmt -s -w

.PHONY: clean
clean:
	@go clean ./...

.PHONY: build
build:
	@go build -o bin/$(PROGRAM) 

execute:
	./bin/$(PROGRAM)

run: build execute

dev: 
	modd

.PHONY: test
test:
	@go test -cover -race ./...

.PHONY: bench
bench:
	@go test -bench=. -benchmem