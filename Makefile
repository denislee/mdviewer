BIN := mdviewer
SAMPLE := README.md

.PHONY: all build run deps clean tidy

all: build

deps:
	go mod tidy

build: deps
	go build -o $(BIN) .

run: build
	./$(BIN) $(if $(FILE),$(FILE),$(SAMPLE))

tidy:
	go mod tidy

clean:
	rm -f $(BIN)
