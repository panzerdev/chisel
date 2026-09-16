VERSION=$(shell git describe --abbrev=0 --tags)
BUILD=$(shell git rev-parse HEAD)
DIRBASE=./build
DIR=${DIRBASE}/${VERSION}/${BUILD}/bin

LDFLAGS=-ldflags "-s -w ${XBUILD} -buildid=${BUILD} -X github.com/jpillora/chisel/share.BuildVersion=${VERSION} -X github.com/panzerdev/chisel/share.BuildVersion=${VERSION} -X main.chiselRepo=https://github.com/panzerdev/chisel"

GOFILES=`go list ./...`
GOFILESNOTEST=`go list ./... | grep -v test`

all:
	@goreleaser build --skip-validate --single-target --config .github/goreleaser.yml

freebsd: lint ${DIR}
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chisel-freebsd_amd64 .

linux: lint ${DIR}
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chisel-linux_amd64 .

windows: lint ${DIR}
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chisel-windows_amd64 .

darwin: ${DIR}
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/chisel-darwin_amd64 .

docker:
	@docker build .

docker-push: ## Build and push docker image with TAG and latest (e.g. make docker-push TAG=v1.0.0)
	@./build-and-push.sh $(TAG)

dep: ## Get the dependencies
	@go install github.com/goreleaser/goreleaser/v2@latest
	@go install github.com/boumenot/gocover-cobertura@latest
	@go mod tidy

lint: ## Lint the files
	@go fmt ${GOFILES}
	@go vet ${GOFILESNOTEST}

${DIR}:
	mkdir -p ${DIR}

test: ${DIR} ## Run unit tests
	@go test -coverprofile=${DIR}/coverage.out -race -short ${GOFILESNOTEST}
	@go tool cover -html=${DIR}/coverage.out -o ${DIR}/coverage.html
	@gocover-cobertura < ${DIR}/coverage.out > ${DIR}/coverage.xml

release: lint test
	goreleaser release --config .github/goreleaser.yml

clean:
	rm -rf ${DIRBASE}/*

run-server: ## Run local server with admin interface
	go run main.go server --port 8080 --admin 127.0.0.1:9000 --reverse --verbose

run-client: ## Run local client with admin interface
	go run main.go client --admin 127.0.0.1:9001 --verbose http://127.0.0.1:8080 3000:127.0.0.1:8080

.PHONY: all freebsd linux windows docker docker-push dep lint test release clean run-server run-client