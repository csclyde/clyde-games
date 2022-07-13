package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	
	_ "github.com/go-sql-driver/mysql"
)

func feedback(w http.ResponseWriter, r *http.Request) {
	
	db, err := sql.Open("mysql", "root:root@/feedback")
	defer db.Close()

	if err != nil {
        log.Fatal(err)
    }

	var version string
	err2 := db.QueryRow("SELECT VERSION()").Scan(&version)

	if err2 != nil {
        log.Fatal(err2)
    }

	fmt.Fprintf(w, version);

};