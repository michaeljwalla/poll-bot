# poll-bot
# Dependencies
 - Docker
 - Docker Compose plugin (optional)
 - Internet connection (required)
 - Github
 - Brain (optional)
# Setup
1. clone repository
2. setup .env (need at least `PROD_TOKENID/DEV_TOKENID` defined)
3. done
# Building & Running the Container
Do both it w/ docker compose:
```sh
docker-compose up -d --build
```
Or if you didn't install that:
```sh
chmod +x ./docker/build.sh && ./docker/build.sh
docker run -d --env-file .env --restart unless-stopped --name main main
```
# Making Sure it Works
 - Is the bot online?
 - Read logs live:
```sh
docker exec poll-bot-main-1 sh -c 'tail -n 10 -f /root/data/logs/$(ls -t /root/data/logs | head -n 1)'
```
