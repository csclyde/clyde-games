build:
	go build -o main
	systemctl stop goweb
	cp main /var/www/api.clyde.games/
	systemctl start goweb
	systemctl restart nginx
