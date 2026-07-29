package endpoints

import (
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
	requestedPurchaseTypes := []string{}
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/appreviews/3219180" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		query := req.URL.Query()
		purchaseType := query.Get("purchase_type")
		if query.Get("filter") != "summary" || query.Get("language") != "all" || query.Get("num_per_page") != "0" {
			t.Fatalf("unexpected query %s", req.URL.RawQuery)
		}
		requestedPurchaseTypes = append(requestedPurchaseTypes, purchaseType)

		body := `{"success":1,"query_summary":{"num_reviews":10,"review_score":8,"review_score_desc":"Very Positive","total_positive":85,"total_negative":14,"total_reviews":99}}`
		if purchaseType == "all" {
			body = `{"success":1,"query_summary":{"num_reviews":10,"review_score":8,"review_score_desc":"Very Positive","total_positive":450,"total_negative":106,"total_reviews":556}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	summary, err := FetchSteamReviewSummary(client, "3219180")
	if err != nil {
		t.Fatalf("FetchSteamReviewSummary returned error: %v", err)
	}
	if len(requestedPurchaseTypes) != 2 || requestedPurchaseTypes[0] != "steam" || requestedPurchaseTypes[1] != "all" {
		t.Fatalf("unexpected purchase type requests %#v", requestedPurchaseTypes)
	}
	if summary.CountedReviews != 99 || summary.TotalReviews != 556 || summary.PositivePercent != 85 || summary.PositivePercentRaw < 85.85 || summary.PositivePercentRaw > 85.86 || summary.ReviewScoreTag != "Very Positive" {
		t.Fatalf("unexpected summary %#v", summary)
	}
}
