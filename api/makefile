build:
	go build -o main
	@sh scripts/prepare-env.sh .env /var/www/api.clyde.games/.env
	systemctl stop goweb
	cp main /var/www/api.clyde.games/
	systemctl start goweb
	systemctl restart nginx
