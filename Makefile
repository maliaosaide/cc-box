.PHONY: build build-cli build-gui clean test test-cli test-gui

build: build-cli build-gui

build-cli:
	mkdir -p cli/build/bin
	cd cli && go build -o build/bin/cc-box.exe ./cmd/cc-box/

build-gui:
	cd gui && wails build -clean -nopackage -m -nosyncgomod

test: test-cli test-gui

test-cli:
	cd cli && go test ./...

test-gui:
	cd gui && go test ./...

clean:
	rm -f cli/build/bin/cc-box.exe
	rm -rf gui/build/bin
