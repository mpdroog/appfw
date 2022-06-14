#!/bin/bash

# Build docker container
docker build -t appfw:main .

# Example of running it
docker run -it --rm -p 1337:1337 --env APPFW_LISTEN=:1337 --env APPFW_APIKEY=vqBKCiiZoEUpYBBP appfw:main -v