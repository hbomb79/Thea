# Change these variables as necessary.
BINARY_NAME := thea

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

.PHONY: fix
fix: build
	go mod tidy -v
	golangci-lint run --fix

.PHONY: lint
lint:
	golangci-lint run

## audit: run quality control checks
.PHONY: audit
audit: tidy
	go generate ./...
	go mod verify
	go vet ./...
	golangci-lint run
	go run honnef.co/go/tools/cmd/staticcheck@latest -f stylish -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go test -buildvcs -vet=off ./...

.PHONY: test
test: build
	go test --count=1 -p=1 -v ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
# .PHONY: test
# test:
# 	go test -v -race -buildvcs ./...

## clean: remove existing artifcats generated by this makefile (.bin)
.PHONY: clean
clean:
	rm -rf ./.bin/

## build: build the application
.PHONY: build
build: 
	go generate ./...
	go build -o=.bin/${BINARY_NAME}

## run: run the  application
.PHONY: run
run: build
	.bin/${BINARY_NAME}

## run/live: run the application with reloading on file changes
.PHONY: run/live
run/live:
	go run github.com/makiuchi-d/arelo@latest \
	--pattern '**/*.go' \
	--pattern '**/*.yaml' \
	--pattern '**/*.tmpl' \
	--ignore '**/*.gen.go' \
	--ignore '**/mocks/*' \
	-- make run
