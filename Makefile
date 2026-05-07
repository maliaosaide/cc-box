.PHONY: build clean test

build:
	go build -o cc-box.exe ./cmd/cc-box/

clean:
	rm -f cc-box.exe

test:
	go test ./internal/...

run: build
	./cc-box.exe $(ARGS)
