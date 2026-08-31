.PHONY : build
.DEFAULT_GOAL := build

OUTPUT_DIR := ../backend/core/root/api/web/dist/

build:
	mkdir -p /tmp/poll-bot/dist/
	export WEB_ROOT_PATH="$$(grep -E '^WEB_ROOT_PATH=' ../.env | cut -d= -f2-)";
	export OUTPUT_DIR=$(OUTPUT_DIR);
		pnpm install --frozen-lockfile;
		pnpm run build