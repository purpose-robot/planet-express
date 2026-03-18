# =========================================================================================================================================== #

include .env
export

# =========================================================================================================================================== #

binary_name = plnx
package_path = ./cmd/api
data_source_name = postgresql://$(DB_POSTGRES_USERNAME):$(DB_POSTGRES_PASSWORD)@$(DB_POSTGRES_HOST):$(DB_POSTGRES_PORT)/$(DB_POSTGRES_NAME)

# =========================================================================================================================================== #

.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

# =========================================================================================================================================== #

.PHONY: audit
audit:
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)"
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...

.PHONY: upgradeable
upgradeable:
	@go run github.com/oligot/go-mod-upgrade@latest

# =========================================================================================================================================== #

.PHONY: tidy
tidy:
	go mod tidy -v
	go fmt ./...
	go fix ./...

.PHONY: build
build:
	go build -o=/tmp/bin/${binary_name} ${package_path}

.PHONY: run
run:
	go run github.com/air-verse/air@latest \
		--build.entrypoint "/tmp/bin/${binary_name}" --build.cmd "make build" --build.delay "100" \
		--build.include_ext "go, tmpl, html, css, js, ts, sql" --env_files ".env" --misc.clean_on_exit "true"

# =========================================================================================================================================== #

.PHONY: migrations/up
migrations/up:
	go run -tags '$(DB_KIND)' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./migrations -database="${data_source_name}" up

.PHONY: migrations/down
migrations/down:
	go run -tags '$(DB_KIND)' github.com/golang-migrate/migrate/v4/cmd/migrate@latest -path=./migrations -database="${data_source_name}" down

# =========================================================================================================================================== #
