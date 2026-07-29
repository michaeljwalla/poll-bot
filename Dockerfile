# use build.sh to set
ARG GO_VER=tip
FROM golang:${GO_VER}-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
COPY src/ ./src/
RUN CGO_ENABLED=0 go build -o main

# other files
COPY . .

# runner
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main ./
COPY docker/img_setup.sh ./
RUN chmod +x ./img_setup.sh && ./img_setup.sh

CMD ["sh", "-c", "MODE=DEV ./main"]
