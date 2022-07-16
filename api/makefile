build:
	go build -o main

deploy:
	cp main /var/www/api.clyde.games/

restart:
	systemctl restart nginx goweb

finish:
	echo "project built and deployed"