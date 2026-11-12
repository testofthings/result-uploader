.PHONY: build
APP_NAME := result-uploader

build:
	go build -o bin/${APP_NAME} cmd/uploader/main.go
