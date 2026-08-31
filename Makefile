.PHONY : init built up down logs dev update web
.DEFAULT_GOAL := build # -> just make

# setup data/ and link env
init:
	@if [ ! -f .env ]; then \
		echo ".env file not found"; \
		exit 1; \
	fi
	mkdir -p ./data/logs/ ./data/polls/


# Compiles the SPA where go:embed picks it up. Run this
# before `go build`/`make dev` to serve the real pages off the Go binary
web:
	$(MAKE) -C ./frontend/

build: init
	$(MAKE) -C ./backend/ build

all: init web build

up: init
	$(MAKE) -C ./backend/ up

update:
	$(MAKE) down
	git pull
	$(MAKE)

down:
	$(MAKE) -C ./backend/ down

logs:
	$(MAKE) -C ./backend/ logs

# add env vars and overwrite MODE
dev: init
	ln -srf ./data ./backend/core/data
	export $$(grep -v '^#' .env | xargs) && \
		cd backend/core && \
			go run -ldflags "-X 'poll-bot/root/info/version.version=dev' -X 'poll-bot/root/info/version.source=local'" .