.PHONY: test build check package
test:
	cd service && go test -race ./...
build:
	cd service && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$$(cat ../VERSION)" -o ../dist/packwiz-manager ./cmd/packwiz-manager
check:
	cd service && test -z "$$(gofmt -l .)" && go vet ./... && go test -race ./...
	bash -n installer/*.sh scripts/*.sh
package: build
	bash scripts/package-release.sh
