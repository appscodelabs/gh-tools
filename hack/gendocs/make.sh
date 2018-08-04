#!/usr/bin/env bash

pushd $GOPATH/src/github.com/appscodelabs/gh-tools/hack/gendocs
go run main.go
popd
