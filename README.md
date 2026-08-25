# poll-bot
## Dependencies
 - [**`make`**](https://www.google.com/search?q=what%20is%20build%20essential)
 - [**`Docker`**](https://www.docker.com/)
 - [**`go`**](https://go.dev/) (if you're testing)

## Setup
1. clone repo
2. insert .env and define `x_TOKENID` where `MODE=X`
```sh
# example .env
MODE=DEV
DEV_TOKENID="..."
```
3. `make` to build image & setup bound data

## Running the Bot
```sh
# docker image
make up

# or with local changes
make dev
```

## Updating
- the bot will check for updates on-startup and via webhook POSTs (if you set that up)
```sh
# add this to your .env to auto-update non breaking changes
REDEPLOY=1

# manually...
make update
make up     # to restart
```

## Overview of Make
```sh
# using
make up     # start
make down   # stop

# updates
make        # for fresh clones
make update # this pulls first


# debug
make logs   # follow output live
make dev    # uses go run instead of docker, MODE=DEV
```
## Hosting the web UI

The frontend compiles to a static SPA that is embedded into the Go binary, so a
deployment is still one artifact and one process. Go serves both the pages and
the API on `SERVER_PORT`.

`WEB_ROOT_PATH` is the path prefix the app is mounted at. It is read by Go at
runtime **and** baked into the frontend at build time, so the two must match —
the server refuses to serve a bundle built for a different prefix and logs why.
Leave it empty if the app owns the whole origin.

```sh
make web    # compile the SPA into root/api/web/dist (go:embed reads it)
make dev    # then run; skip `make web` and you get a "not built" notice page
```

Releases build the frontend in CI, so the published binary already contains it.
That step reads the repository variable `WEB_ROOT_PATH` — set it under
Settings → Secrets and variables → Actions → Variables, or tagged builds ship a
bundle compiled for `/`.

### Behind a Cloudflare tunnel

`cloudflared` forwards the matched path to the origin **without stripping it**,
so a tunnel serving `my-site.com/pb` must be paired with `WEB_ROOT_PATH=/pb`.

```yaml
# ~/.cloudflared/config.yml
tunnel: <TUNNEL-ID>
credentials-file: /root/.cloudflared/<TUNNEL-ID>.json

ingress:
  - hostname: my-site.com
    path: ^/pb(/.*)?$
    service: http://localhost:10000
  - service: http_status:404
```

`docker compose` binds the port to `127.0.0.1` only, so the origin is reachable
by the tunnel but not from the LAN.
