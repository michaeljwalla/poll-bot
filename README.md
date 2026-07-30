# poll-bot
## Dependencies
 - make
 - Docker / Docker compose
## Setup
1. clone repo
2. setup .env and define `x_TOKENID` where `MODE=X`
    - ex, `MODE=DEV DEV_TOKENID=...`
3. done
## Using the Bot
```sh
make        # build (no cache)
make up     # start
make down   # stop
make logs   # follow output live
```