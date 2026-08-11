.PHONY : init built up down logs dev update
.DEFAULT_GOAL := build # -> just make

# setup data/
init:
	@if [ ! -f .env ]; then \
		echo ".env file not found"; \
		exit 1; \
	fi
	mkdir -p ./data/logs/ ./data/polls/misc/ 


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
	export $$(grep -v '^#' .env | xargs); MODE=DEV go run -ldflags "-X 'poll-bot/src/version.version=dev' -X 'poll-bot/src/version.source=local'" .