.PHONY : init built up down logs dev update web
.DEFAULT_GOAL := build # -> just make

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f