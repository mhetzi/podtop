#!/bin/bash
podman run --rm -it -v "./":/usr/src/myapp:z -w /usr/src/myapp docker.io/library/golang:1.26 \
  bash -c '
    ls /usr/src/myapp/src/podtop/ -la
    for GOOS in darwin linux; do
      for GOARCH in arm64 amd64; do
        export GOOS GOARCH
        go build -C /usr/src/myapp/src/podtop/ -v -o "/usr/src/myapp/build/podtop-$GOOS-$GOARCH"
      done
    done
  '
