# Docker build
FROM golang:1.16 AS builder
WORKDIR /app 
COPY . .
RUN go get
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags "-extldflags \"-static\" -X main.version=$(git describe --always --long --dirty --all)-$(date +%Y-%m-%d-%H:%M)" -o appfw
RUN echo "appfw:x:1513:1513:appfw user:/appfw:/dev/null" > passwd; echo "appfw:x:1513:" > group

# Docker run
FROM busybox
LABEL MAINTAINER Mark Droog <rootdev@gmail.com>
WORKDIR /app
COPY --from=builder /app/appfw /usr/bin/
COPY --from=builder /app/passwd /etc/passwd
COPY --from=builder /app/group /etc/group

USER appfw:appfw
EXPOSE 1337/tcp
ENTRYPOINT ["appfw"]

# Need to debug container? Just exec into it
# docker exec -ti container-name sh)
