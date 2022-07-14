package main

import "net/http"
import "github.com/gin-gonic/gin"
import "database/sql"
import _ "github.com/go-sql-driver/mysql"

type Feedback struct {
    Id int `json:"id"`
    Message string `json:"message"`
	Rating int `json:"rating"`
	Created string `json:"created"`
	Env string `json:"env"`
}

func get_feedback(c *gin.Context) {
	// open db connection
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/analytics")

	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	defer db.Close()

	// query all the feedback
	results, err := db.Query("SELECT * FROM feedback")

	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	// convert the results into Feedback structs
	feedback := []Feedback{}

	for results.Next() {
		var fb Feedback
		err := results.Scan(&fb.Id, &fb.Message, &fb.Rating, &fb.Created, &fb.Env)

		if err != nil {
			c.AbortWithError(http.StatusNotFound, err)
			return
		}

		feedback = append(feedback, fb)
	}

	// check the result, and return it as JSON
	if feedback == nil || len(feedback) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	} else {
		c.IndentedJSON(http.StatusOK, feedback)
	}
}

func add_feedback(c *gin.Context) {
	var feedback Feedback

	if err := c.BindJSON(&feedback); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	} 

	// open db connection
	db, err := sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/analytics")

	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return
	}

	defer db.Close()

	// insert the new feedback item
	insert, err := db.Query(
		"INSERT INTO feedback (message,rating) VALUES (?,?)",
		feedback.Message, feedback.Rating)

	if err != nil {
		c.AbortWithError(http.StatusNotFound, err)
		return

	}

	defer insert.Close()

	c.IndentedJSON(http.StatusCreated, feedback)
}
