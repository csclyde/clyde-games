.PHONY: build test clean
build:
	go build -trimpath -o bin/clyde-games-mcp .
test:
	go test ./...
	go vet ./...
clean:
	rm -f bin/clyde-games-mcp
