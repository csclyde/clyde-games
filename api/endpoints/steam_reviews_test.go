package endpoints

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchSteamReviewSummary(t *testing.T) {
	requestedPurchaseTypes := []int{}
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.steampowered.com" || req.URL.Path != "/IUserReviewsService/GetAppReviews/v1/" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		var query struct {
			AppID        uint32   `json:"appid"`
			Filter       int      `json:"filter"`
			Languages    []string `json:"languages"`
			PurchaseType int      `json:"purchase_type"`
			NumPerPage   int      `json:"num_per_page"`
		}
		if err := json.Unmarshal([]byte(req.URL.Query().Get("input_json")), &query); err != nil {
			t.Fatal(err)
		}
		purchaseType := query.PurchaseType
		if query.AppID != 3219180 || query.Filter != 1 || len(query.Languages) != 1 || query.Languages[0] != "all" || query.NumPerPage != 1 {
			t.Fatalf("unexpected query %+v", query)
		}
		requestedPurchaseTypes = append(requestedPurchaseTypes, purchaseType)

		body := `{"success":1,"query_summary":{"num_reviews":10,"review_score":8,"review_score_desc":"Very Positive","total_positive":85,"total_negative":14,"total_reviews":99}}`
		if purchaseType == 1 {
			body = `{"success":1,"query_summary":{"num_reviews":10,"review_score":8,"review_score_desc":"Very Positive","total_positive":450,"total_negative":106,"total_reviews":556}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"response":` + body + `}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	summary, err := FetchSteamReviewSummary(client, "3219180")
	if err != nil {
		t.Fatalf("FetchSteamReviewSummary returned error: %v", err)
	}
	if len(requestedPurchaseTypes) != 2 || requestedPurchaseTypes[0] != 0 || requestedPurchaseTypes[1] != 1 {
		t.Fatalf("unexpected purchase type requests %#v", requestedPurchaseTypes)
	}
	if summary.CountedReviews != 99 || summary.TotalReviews != 556 || summary.PositivePercent != 85 || summary.PositivePercentRaw < 85.85 || summary.PositivePercentRaw > 85.86 || summary.ReviewScoreTag != "Very Positive" {
		t.Fatalf("unexpected summary %#v", summary)
	}
}

func TestFetchSteamReviewsPage(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var input map[string]interface{}
		if err := json.Unmarshal([]byte(req.URL.Query().Get("input_json")), &input); err != nil {
			t.Fatal(err)
		}
		if input["cursor"] != "next+/=" || input["filter"] != float64(1) || input["purchase_type"] != float64(1) || input["num_per_page"] != float64(100) {
			t.Fatalf("unexpected pagination/filter inputs: %#v", input)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"response":{"cursor":"another+/=","reviews":[{"recommendationid":"123","author":{"steamid":"456"},"review":"Great","voted_up":true,"timestamp_created":1794356285}]}}`))}, nil
	})}
	page, err := fetchSteamReviewsPage(client, "3219180", "next+/=", 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Cursor != "another+/=" || len(page.Reviews) != 1 || page.Reviews[0].RecommendationID != "123" || page.Reviews[0].Author.SteamID != "456" {
		t.Fatalf("unexpected review page: %#v", page)
	}
}

func TestFetchSteamReviewsErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"rate limit", 429, "Too many requests", "429"},
		{"HTML redirect", 200, "<html>Steam</html>", "invalid JSON"},
		{"missing response", 200, `{}`, "missing review data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Status: fmt.Sprintf("%d %s", tc.status, http.StatusText(tc.status)), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			_, err := fetchSteamReviews("https://example.test/reviews", client)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}
