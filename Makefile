all: build

build:
	go build -o gphotos-fb cmd/main.go

run:
	go run cmd/main.go
