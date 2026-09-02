.PHONY: build run

# Linux on Ubuntu 24: make TAGS=production,webkit2_41
TAGS ?= production

build:
	go build -tags $(TAGS) -o Tailsend .

run: build
	./Tailsend
