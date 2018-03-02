#!/bin/sh

os=`uname -s`
arch=`uname -m`

# TODO: Cross compile via docker / use curl if Go is not installed.
go build ./cmd/...

echo "helm-hashtag installed!"
echo "See the README for usage examples."
echo
