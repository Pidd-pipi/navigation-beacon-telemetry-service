.PHONY: build vet fmt test race run linux-build docker-build docker-run clean

build:
	go build ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

test:
	go test ./...

race:
	go test -race ./...

run:
	go run .

linux-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o server .

docker-build:
	docker build -t navigation-beacon-telemetry-service:latest .

docker-run:
	docker run --rm -p 8080:8080 navigation-beacon-telemetry-service:latest

clean:
	rm -f server navigation-beacon-telemetry-service
