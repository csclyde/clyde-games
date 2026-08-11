build:
	go build -o main
	systemctl stop goweb
	cp main /var/www/api.clyde.games/
	packrat_token=$$(sed -n 's/^PACKRAT_DEPLOY_TOKEN=//p' /var/www/api.clyde.games/.env); cp .env /var/www/api.clyde.games/.env; if [ -n "$$packrat_token" ]; then printf '\nPACKRAT_DEPLOY_TOKEN=%s\n' "$$packrat_token" >> /var/www/api.clyde.games/.env; fi
	systemctl start goweb
	systemctl restart nginx
