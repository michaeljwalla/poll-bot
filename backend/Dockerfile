# use build.sh to set
ARG GO_VER=tip
FROM golang:${GO_VER}-alpine AS downloader

WORKDIR /app

# jq parses the github api
RUN apk add --no-cache curl jq

#formalities
ARG USERNAME="michaeljwalla"
ARG REPO="poll-bot"
ARG TARGET="linux-amd64"

# what the hell are you doing jq
# gets (pre)release builds
RUN DOWNLOAD_URL=$(curl -sSL \
      "https://api.github.com/repos/${USERNAME}/${REPO}/releases" \
      | jq -r '[.[]][0].assets[] | select(.name | test("'"${TARGET}"'")) | .browser_download_url') \
    && curl -sSL "${DOWNLOAD_URL}" -o /tmp/release.tar.gz \
    && mkdir -p ./bin \
    && tar -xvzf /tmp/release.tar.gz -C ./bin

# runner
FROM alpine:latest
WORKDIR /app

COPY --from=downloader /app/bin/main ./main

CMD ["./main"]
