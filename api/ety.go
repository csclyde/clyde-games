package main

import (
	"fmt"
	"net/http"
	"encoding/json"
    "io/ioutil"
	"log"
	
)

func ety(w http.ResponseWriter, r *http.Request) {
	
	content, err := ioutil.ReadFile("./etymologies.json")
    if err != nil {
		log.Fatal("Error when opening file: ", err)
    }
	
	var payload map[string]interface{}
    err = json.Unmarshal(content, &payload)
    if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
    }
	
	var word = r.URL.Query().Get("word")
	
	// log.Printf("origin: %s\n", payload["house"])
	fmt.Fprintf(w, word + ":: ");
};