build:
	go build -o main

deploy: main
	cp main /var/www/api.clyde.games/

restart:
	systemctl restart nginx goweb

finish:
	echo "project built and deployed"