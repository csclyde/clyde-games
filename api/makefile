build:
	go build -o main
	systemctl stop goweb
	cp main /var/www/api.clyde.games/
	cp .env /var/www/api.clyde.games/.env
	systemctl start goweb
	systemctl restart nginx
