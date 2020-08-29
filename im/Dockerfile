FROM golang:alpine as builder

RUN mkdir /app
WORKDIR /app

ENV GO111MODULE=on
ENV GOPROXY="https://goproxy.io"

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sx-im im/*.go

# Run container
FROM alpine:latest

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates

RUN mkdir /app
RUN mkdir -p /data/im/pending
WORKDIR /app

COPY --from=builder /app/sx-im .

ENTRYPOINT ["/app/sx-im"]

