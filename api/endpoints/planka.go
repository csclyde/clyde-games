package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type plankaItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type plankaBoard struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	ProjectID string       `json:"projectId"`
	Lists     []plankaItem `json:"lists"`
}

type plankaProject struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Boards []plankaBoard `json:"boards"`
}

func plankaConfig() (string, string, string, error) {
	baseURL := strings.TrimRight(os.Getenv("PLANKA_URL"), "/")
	username := os.Getenv("PLANKA_USERNAME")
	password := os.Getenv("PLANKA_PASSWORD")
	if baseURL == "" || username == "" || password == "" {
		return "", "", "", fmt.Errorf("PLANKA_URL, PLANKA_USERNAME, and PLANKA_PASSWORD must be configured")
	}
	return baseURL, username, password, nil
}

func plankaRequest(method, path string, body interface{}) ([]byte, error) {
	baseURL, username, password, err := plankaConfig()
	if err != nil {
		return nil, err
	}

	loginBody, _ := json.Marshal(gin.H{"emailOrUsername": username, "password": password})
	loginResponse, err := http.Post(baseURL+"/api/access-tokens", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		return nil, fmt.Errorf("could not connect to Planka: %w", err)
	}
	defer loginResponse.Body.Close()
	loginData, _ := io.ReadAll(loginResponse.Body)
	if loginResponse.StatusCode < 200 || loginResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("Planka login failed (%d): %s", loginResponse.StatusCode, strings.TrimSpace(string(loginData)))
	}
	var login struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(loginData, &login); err != nil || login.Item == "" {
		return nil, fmt.Errorf("Planka login returned no access token")
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, baseURL+path, requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+login.Item)
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Planka request failed: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Planka request failed (%d): %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func GetPlankaHierarchy(c *gin.Context) {
	data, err := plankaRequest(http.MethodGet, "/api/projects", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	var response struct {
		Items    []plankaProject `json:"items"`
		Included struct {
			Boards []plankaBoard `json:"boards"`
		} `json:"included"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Could not read Planka projects"})
		return
	}
	for i := range response.Items {
		for _, board := range response.Included.Boards {
			if board.ProjectID != response.Items[i].ID {
				continue
			}
			boardData, requestErr := plankaRequest(http.MethodGet, "/api/boards/"+board.ID, nil)
			if requestErr != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": requestErr.Error()})
				return
			}
			var boardResponse struct {
				Item     plankaBoard `json:"item"`
				Included struct {
					Lists []plankaItem `json:"lists"`
				} `json:"included"`
			}
			if json.Unmarshal(boardData, &boardResponse) == nil {
				board.Lists = boardResponse.Included.Lists
			}
			response.Items[i].Boards = append(response.Items[i].Boards, board)
		}
	}
	c.JSON(http.StatusOK, response.Items)
}

func CreatePlankaTicket(c *gin.Context) {
	var input struct{ ListID, Name, Description string }
	if err := c.BindJSON(&input); err != nil || input.ListID == "" || strings.TrimSpace(input.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listId and name are required"})
		return
	}
	data, err := plankaRequest(http.MethodPost, "/api/lists/"+input.ListID+"/cards", gin.H{"name": input.Name, "description": input.Description, "position": 0})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	var result interface{}
	if json.Unmarshal(data, &result) != nil {
		c.JSON(http.StatusCreated, gin.H{"created": true})
		return
	}
	c.JSON(http.StatusCreated, result)
}
