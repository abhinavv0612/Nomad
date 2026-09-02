package main

import (
	"net/http"
)

// GET /api/google/search?q=Baga+Beach+Goa
// Returns candidate places from Google's live Text Search.
func (srv *Server) handleGoogleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"method not allowed"})
		return
	}
	if srv.googlePlaces == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{"Google Places not configured: set GOOGLE_PLACES_API_KEY"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"q is required"})
		return
	}

	candidates, err := srv.googlePlaces.SearchText(query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

// GET /api/google/place?place_id=ChIJ...
// Returns live details + reviews for a specific place_id.
// Nothing here is persisted — see google_places.go for why.
func (srv *Server) handleGoogleDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"method not allowed"})
		return
	}
	if srv.googlePlaces == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{"Google Places not configured: set GOOGLE_PLACES_API_KEY"})
		return
	}

	placeID := r.URL.Query().Get("place_id")
	if placeID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"place_id is required"})
		return
	}

	details, err := srv.googlePlaces.GetDetails(placeID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, details)
}
