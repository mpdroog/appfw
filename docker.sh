#!/bin/bash
set -x

# Build docker container
docker build -t appfw:main .
docker image tag appfw:main mpdroog/appfw:main
docker push mpdroog/appfw:main

# Example of running it
# docker run -it --rm -p 1337:1337 --env APPFW_LISTEN=:1337 --env APPFW_APIKEY=vqBKCiiZoEUpYBBP appfw:main -v
