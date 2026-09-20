DESTDIR :=
PREFIX := /usr/local

.PHONY: all build fmt-check vet test verify smoke
all: build

build:
	go build -o bin/tt src/*.go

fmt-check:
	test -z "$$(gofmt -l src/*.go)"

vet:
	go vet ./...

test:
	go test ./...

verify: fmt-check vet test build

smoke: build
	./bin/tt -list words
tt.1.gz: man.md
	pandoc -s -t man -o - man.md|gzip > tt.1.gz

.PHONY: install
install: build tt.1.gz
	install -d $(DESTDIR)$(PREFIX)/bin
	install -d $(DESTDIR)$(PREFIX)/share/man/man1
	install -m755 bin/tt $(DESTDIR)$(PREFIX)/bin
	install -m644 tt.1.gz $(DESTDIR)$(PREFIX)/share/man/man1

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/tt
	rm -f $(DESTDIR)$(PREFIX)/share/man/man1/tt.1.gz

.PHONY: assets
assets:
	python3 ./scripts/themegen.py
	./scripts/pack themes/ words/ quotes/ sounds/ | gofmt > src/packed.go
	pandoc -s -t man -o - man.md|gzip > tt.1.gz

.PHONY: rel
rel:
	GOOS=darwin GOARCH=amd64 go build -o bin/tt-osx src/*.go
	GOOS=windows GOARCH=amd64 go build -o bin/tt.exe src/*.go
	GOOS=linux GOARCH=amd64 go build -o bin/tt-linux src/*.go
	GOOS=linux GOARCH=arm go build -o bin/tt-linux_arm src/*.go
	GOOS=linux GOARCH=arm64 go build -o bin/tt-linux_arm64 src/*.go

