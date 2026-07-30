# use build.sh to set
ARG GO_VER=tip
FROM golang:${GO_VER}-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY src/ ./src/
RUN CGO_ENABLED=0 go build -o main

COPY docker/img_setup.sh ./

# runner
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main /app/img_setup.sh ./
RUN ./img_setup.sh


CMD ["sh", "-c", "MODE=DEV ./main"]
