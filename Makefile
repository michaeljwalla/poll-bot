.PHONY : init built up down logs dev update web
.DEFAULT_GOAL := build # -> just make

# setup data/
init:
	@if [ ! -f .env ]; then \
		echo ".env file not found"; \
		exit 1; \
	fi
	mkdir -p ./data/logs/ ./data/polls/


# Compiles the SPA into root/api/web/dist, where go:embed picks it up. Run this
# before `go build`/`make dev` to serve the real pages off the Go binary
web:
	cd frontend && WEB_ROOT_PATH="$$(grep -E '^WEB_ROOT_PATH=' ../.env | cut -d= -f2-)" pnpm install --frozen-lockfile && \
		WEB_ROOT_PATH="$$(grep -E '^WEB_ROOT_PATH=' ../.env | cut -d= -f2-)" pnpm run build

build: init
	docker compose build

up: init
	docker compose up -d

update:
	$(MAKE) down
	git pull
	$(MAKE) build

down:
	docker compose down

# condition passes keyb sigint
logs:
	@docker compose logs -f || [ $$? -eq 130 ]

# add env vars and overwrite MODE
dev: init
	export $$(grep -v '^#' .env | xargs); go run -ldflags "-X 'poll-bot/src/version.version=dev' -X 'poll-bot/src/version.source=local'" .