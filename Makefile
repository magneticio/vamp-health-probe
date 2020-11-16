SHELL             := bash
.SHELLFLAGS       := -eu -o pipefail -c
.DEFAULT_GOAL     := default
.DELETE_ON_ERROR  :
.SUFFIXES         :# Go parameters

GOCMD	:= go
GOTEST 	:= $(GOCMD)	test

.PHONY: test
test:
	$(GOTEST) ./...