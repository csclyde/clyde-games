package endpoints

import (
	"net/http"
	"regexp"
	"strings"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

type WordRequest struct {
	Text string
}

func AnalyzeWords(c *gin.Context) {
	// get the words from the request body
	var request WordRequest
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// filter out non-word characters from the text
	processedString, err := stripSymbols(request.Text)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	processedString = strings.ToLower(processedString)
	wordArray := strings.Split(processedString, " ")
	uniqueWordArray := removeDuplicateWords((wordArray))

	// grab all those words from the DB
	queriedWords, err := models.SelectWords(uniqueWordArray)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// find any remaining words that dont exist in the DB
	var remainingWords []string

	for _, w := range uniqueWordArray {
		wordFound := false

		for _, z := range queriedWords {
			if z.Text == w {
				wordFound = true
				break
			}
		}

		if !wordFound {
			remainingWords = append(remainingWords, w)
		}
	}

	// create new word objects for them
	var newWordList []models.Word
	for _, w := range remainingWords {
		var newWord models.Word
		newWord.Text = w
		newWord.Origin = models.Unknown
		newWordList = append(newWordList, newWord)
	}

	// add all those new word objects to the DB
	newWordList, err = models.AddWords(newWordList)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// assemble a word array by going through the initial list of words
	var analyzedText []models.Word
	completeWordList := append(queriedWords, newWordList...)
	for _, w := range wordArray {
		for _, q := range completeWordList {
			if q.Text == w {
				analyzedText = append(analyzedText, q)
				continue
			}
		}
	}

	c.IndentedJSON(http.StatusCreated, analyzedText)
}

func stripSymbols(text string) (string, error) {
	reg, err := regexp.Compile(`[^\w]`)
	if err != nil {
		return "", err
	}

	return reg.ReplaceAllString(text, " "), nil
}

func removeDuplicateWords(words []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	// If the key(values of the slice) is not equal
	// to the already present value in new slice (list)
	// then we append it. else we jump on another element.
	for _, entry := range words {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
