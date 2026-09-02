package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// This file talks live to Google's Places API (New) on every request.
// Per Google's Places API policies, content (ratings, review text, photos,
// names) must NOT be cached or stored beyond narrow exceptions (place_id
// indefinitely, lat/lng for 30 days). So this client fetches fresh every
// time and returns straight through to the caller — nothing here writes
// to our own database.

const (
	placesSearchURL = "https://places.googleapis.com/v1/places:searchText"
	placesDetailURL = "https://places.googleapis.com/v1/places/%s" // %s = place_id
)

type GooglePlacesClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewGooglePlacesClient() (*GooglePlacesClient, error) {
	key := os.Getenv("GOOGLE_PLACES_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GOOGLE_PLACES_API_KEY is not set")
	}
	return &GooglePlacesClient{
		apiKey:     key,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}, nil
}

// ---------- Text Search: find candidate places for a free-text query ----------

type searchTextRequest struct {
	TextQuery string `json:"textQuery"`
}

type searchTextCandidate struct {
	ID   string `json:"id"`
	Name string `json:"displayName.text"` // not used directly, see custom unmarshal below
}

// The New Places API nests displayName as {"text": "...", "languageCode": "en"}.
type displayName struct {
	Text string `json:"text"`
}

type searchTextResultRaw struct {
	ID          string      `json:"id"`
	DisplayName displayName `json:"displayName"`
	Rating      float64     `json:"rating"`
	UserRatingC int         `json:"userRatingCount"`
}

type searchTextResponseRaw struct {
	Places []searchTextResultRaw `json:"places"`
}

type PlaceCandidate struct {
	PlaceID     string  `json:"place_id"`
	Name        string  `json:"name"`
	Rating      float64 `json:"rating"`
	RatingCount int     `json:"rating_count"`
}

// SearchText finds candidate places matching a free-text query (e.g. "Baga Beach Goa").
func (c *GooglePlacesClient) SearchText(query string) ([]PlaceCandidate, error) {
	body, err := json.Marshal(searchTextRequest{TextQuery: query})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, placesSearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	// Field mask keeps the response small and keeps you on cheaper SKU tiers —
	// only ask Google for fields you actually use.
	req.Header.Set("X-Goog-FieldMask", "places.id,places.displayName,places.rating,places.userRatingCount")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("places searchText failed: %s: %s", resp.Status, string(raw))
	}

	var parsed searchTextResponseRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	candidates := make([]PlaceCandidate, 0, len(parsed.Places))
	for _, p := range parsed.Places {
		candidates = append(candidates, PlaceCandidate{
			PlaceID:     p.ID,
			Name:        p.DisplayName.Text,
			Rating:      p.Rating,
			RatingCount: p.UserRatingC,
		})
	}
	return candidates, nil
}

// ---------- Place Details: full info incl. reviews, open-now status ----------

type authorAttribution struct {
	DisplayName string `json:"displayName"`
	URI         string `json:"uri"`
	PhotoURI    string `json:"photoUri"`
}

type reviewRaw struct {
	Name                    string            `json:"name"`
	Rating                  int               `json:"rating"`
	Text                    displayName       `json:"text"`
	RelativePublishTimeDesc string            `json:"relativePublishTimeDescription"`
	AuthorAttribution       authorAttribution `json:"authorAttribution"`
	PublishTime             string            `json:"publishTime"`
}

type openingHoursRaw struct {
	OpenNow bool `json:"openNow"`
}

type placeDetailsRaw struct {
	ID                  string          `json:"id"`
	DisplayName         displayName     `json:"displayName"`
	Rating              float64         `json:"rating"`
	UserRatingCount     int             `json:"userRatingCount"`
	GoogleMapsURI       string          `json:"googleMapsUri"`
	CurrentOpeningHours openingHoursRaw `json:"currentOpeningHours"`
	FormattedAddress    string          `json:"formattedAddress"`
	Reviews             []reviewRaw     `json:"reviews"`
}

type LiveReview struct {
	Rating           int    `json:"rating"`
	Text             string `json:"text"`
	RelativeTime     string `json:"relative_time"` // e.g. "3 weeks ago"
	AuthorName       string `json:"author_name"`
	AuthorPhotoURL   string `json:"author_photo_url"`
	AuthorProfileURL string `json:"author_profile_url"`
}

type LivePlaceDetails struct {
	PlaceID       string       `json:"place_id"`
	Name          string       `json:"name"`
	Address       string       `json:"address"`
	Rating        float64      `json:"rating"`
	RatingCount   int          `json:"rating_count"`
	OpenNow       bool         `json:"open_now"`
	GoogleMapsURL string       `json:"google_maps_url"`
	Reviews       []LiveReview `json:"reviews"`
	// Attribution is required by Google's policy whenever you display this data.
	Attribution string `json:"attribution"`
}

// GetDetails fetches live details + reviews for a given place_id.
// place_id is the ONE piece of Google data you're allowed to store
// indefinitely — everything else in this struct must be re-fetched on
// every read, never persisted.
func (c *GooglePlacesClient) GetDetails(placeID string) (*LivePlaceDetails, error) {
	url := fmt.Sprintf(placesDetailURL, placeID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", "id,displayName,rating,userRatingCount,googleMapsUri,currentOpeningHours,formattedAddress,reviews")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("places details failed: %s: %s", resp.Status, string(raw))
	}

	var parsed placeDetailsRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	reviews := make([]LiveReview, 0, len(parsed.Reviews))
	for _, r := range parsed.Reviews {
		reviews = append(reviews, LiveReview{
			Rating:           r.Rating,
			Text:             r.Text.Text,
			RelativeTime:     r.RelativePublishTimeDesc,
			AuthorName:       r.AuthorAttribution.DisplayName,
			AuthorPhotoURL:   r.AuthorAttribution.PhotoURI,
			AuthorProfileURL: r.AuthorAttribution.URI,
		})
	}

	return &LivePlaceDetails{
		PlaceID:       parsed.ID,
		Name:          parsed.DisplayName.Text,
		Address:       parsed.FormattedAddress,
		Rating:        parsed.Rating,
		RatingCount:   parsed.UserRatingCount,
		OpenNow:       parsed.CurrentOpeningHours.OpenNow,
		GoogleMapsURL: parsed.GoogleMapsURI,
		Reviews:       reviews,
		Attribution:   "Ratings and reviews via Google",
	}, nil
}
