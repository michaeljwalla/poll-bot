# poll-bot
## Dependencies
 - make
 - Docker
## Setup
1. clone repo
2. setup .env and define `x_TOKENID` where `MODE=X`
    - ex, `MODE=DEV DEV_TOKENID=...`
3. done
## Using Make
```sh
# updates
make        # (re)build, fetches latest release

# using
make up     # start
make down   # stop

# debug
make logs   # follow output live
make dev    # uses go run instead of docker, MODE=DEV
```