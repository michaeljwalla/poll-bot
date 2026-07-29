# poll-bot
# Dependencies
 - Docker
 - Docker Compose plugin (optional)
 - Internet connection
 - Github
 - Brain
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