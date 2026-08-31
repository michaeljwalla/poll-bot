# poll-bot
## Dependencies
 - [**`make`**](https://www.google.com/search?q=what%20is%20build%20essential)
 - [**`Docker`**](https://www.docker.com/)
 - [**`go`**](https://go.dev/) (if you're testing)
 - [**`node`**](https://nodejs.org/) (if using webserver)
 - [**`pnpm`**](https://pnpm.io/) (if using webserver)
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

The frontend compiles to a static SPA embedded into the Go binary, so a
deployment is still one artifact and one process. Go serves both the pages and
the API on `SERVER_PORT`.

```sh
make web    # compile the SPA (go:embed reads it)
make dev    # then run; skip `make web` and you get a "not built" notice page
```

Releases build the frontend in CI, so the published binary already contains it.