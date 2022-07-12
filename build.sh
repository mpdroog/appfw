#!/bin/bash
set -e
set -x
set -u

env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags "-extldflags \"-static\" -X main.version=$(git describe --always --long --dirty --all)-$(date +%Y-%m-%d-%H:%M)" -o appfw
echo -n $'\003' | dd bs=1 count=1 seek=7 conv=notrunc of=./afd
