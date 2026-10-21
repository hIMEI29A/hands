GO ?= env -u GOROOT go

.PHONY: build vet fmt test test-short lint check

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

fmt:
	@files="$$(gofmt -l src)"; \
	if [ -n "$$files" ]; then \
		echo "These files are not gofmt'd:"; \
		echo "$$files"; \
		exit 1; \
	fi

test:
	$(GO) test ./... -race -count=1

test-short:
	$(GO) test ./... -short -count=1

lint:
	env -u GOROOT golangci-lint run

check: fmt build vet test
