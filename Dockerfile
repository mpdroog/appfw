# Docker build
FROM golang:alpine as builder
WORKDIR /app 
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o="appfw" -ldflags="-w -s" .

# Docker run
FROM busybox
WORKDIR /app
COPY --from=builder /app/appfw /usr/bin/
EXPOSE 1337/tcp
ENTRYPOINT ["appfw"]

# Need to debug container? Just exec into it
# docker exec -ti container-name sh)
