SHELL := sh

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' 

.PHONY: build
build: ## build module
	go build ./...

.PHONY: fuzz
fuzz: ## fuzz test
	go test ./cmd/diff/ -fuzz=FuzzUnifiedParity -fuzztime=5m

.PHONY: test
test: ## run all unit tests
	go test ./...

.PHONY: version
version: ## print OS, Go, and golangci versions
	@echo $$0
	@uname -a
	@go version
	@golangci-lint --version

.PHONY: cover
cover: ## generate code coverage report
	rm -f cover.out
	go test -run='^Test' -coverprofile=cover.out -coverpkg=.
	go tool cover -func=cover.out

.PHONY: lintverify
## NOTE: this downloads it's schema over the network
lintverify:
	golangci-lint config verify

.PHONY: fmt
fmt: ## reformat source code
	go mod tidy
	gofmt -w -s .

.PHONY: lint
lint: ## lint and verify repo is already formatted
	go mod tidy
	git diff --exit-code -- go.mod go.sum
	golangci-lint run ./...

.PHONY: clean
clean: ## remove any generated files
	rm -f *.out 
	rm -f ./diff

