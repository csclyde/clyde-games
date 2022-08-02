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

type WordResponse struct {
	Words      []models.Word
	Statistics Stats
}

type Stats struct {
	Total    int
	Unknown  int
	Germanic int
	French   int
	Latin    int
	Greek    int
	Other    int
}

func GetUnknownWords(c *gin.Context) {
	unknowns, err := models.SelectUnknownWords()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, unknowns)
}

func GetAllWordStats(c *gin.Context) {
	words, err := models.SelectAllWords()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, GenerateStats(words))
}

func GenerateStats(words []models.Word) Stats {
	var stats Stats

	stats.Total = len(words)

	for _, w := range words {
		switch w.Origin {
		case 0:
			stats.Unknown += 1
		case 1:
			stats.Germanic += 1
		case 2:
			stats.French += 1
		case 3:
			stats.Latin += 1
		case 4:
			stats.Greek += 1
		case 5:
			stats.Other += 1
		default:
			stats.Unknown += 1
		}
	}

	return stats
}

func UpdateWords(c *gin.Context) {
	var words []models.Word

	if err := c.BindJSON(&words); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// open db connection
	updatedWords, err := models.UpdateWords(words)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return

	}

	c.IndentedJSON(http.StatusCreated, updatedWords)
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
	uniqueWordArray := removeDuplicateWords(wordArray)

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

	var resp WordResponse
	resp.Words = analyzedText
	resp.Statistics = GenerateStats(analyzedText)

	c.IndentedJSON(http.StatusCreated, resp)
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
			if entry != "" {
				list = append(list, entry)
			}
		}
	}
	return list
}
