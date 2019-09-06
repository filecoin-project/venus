include $(GOPATH)/src/github.com/filecoin-project/go-filecoin/mk/golang.mk

TARGET=go-filecoin-migrate
SRC := $(shell find . -type f -name '*.go' -not -path "./vendor/*")

all: build

$(TARGET): $(SRC)
	$(GOBUILD) -o $(TARGET)

build: $(TARGET)
	@true

lint:
	@$(GOCMD) run github.com/golangci/golangci-lint/cmd/golangci-lint run

clean:
	@$(GOCLEAN)
	@rm -f $(TARGET)

test: $(TARGET)
	@$(GOTEST) ./...

.PHONY: all build lint clean test
