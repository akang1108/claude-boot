BINARY := claude-boot

.PHONY: build test fmt clean install dist
build:
	go build -o $(BINARY) .

test:
	go test ./...

fmt:
	go fmt ./...

install: test
	go install .

dist:
	GOOS=darwin  GOARCH=arm64  go build -o dist/$(BINARY)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64  go build -o dist/$(BINARY)-linux-amd64 .
	GOOS=windows GOARCH=amd64  go build -o dist/$(BINARY)-windows-amd64.exe .

clean:
	rm -f $(BINARY)
	rm -rf dist/
