build:
	go build -o main
	cp main /var/www/api.clyde.games/
	systemctl restart nginx goweb
	echo "project built and deployed"
