package main

import (
	"encoding/json"
	"net/http"
)

type createTripRequest struct {
	Name string `json:"name"`
}

type addTripPlaceRequest struct {
	Source  string `json:"source"`   // "local" or "google"
	PlaceID string `json:"place_id"` // our own place id, or a Google place_id
	Name    string `json:"name"`     // denormalized display name at time of adding
	Note    string `json:"note"`
}

// POST /api/trips  {"name": "Goa Dec trip"}
func (srv *Server) handleCreateTrip(w http.ResponseWriter, r *http.Request) {
	var req createTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid JSON body"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"name is required"})
		return
	}

	trip := srv.trips.Create(req.Name)
	writeJSON(w, http.StatusCreated, trip)
}

// GET /api/trips  -> list of trips (id, name, place count) — the "My trips" view
func (srv *Server) handleListTrips(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, srv.trips.List())
}

// GET /api/trips/{id}  -> full trip with places
func (srv *Server) handleGetTrip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	trip, ok := srv.trips.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{"trip not found"})
		return
	}
	writeJSON(w, http.StatusOK, trip)
}

// POST /api/trips/{id}/places  {"source":"local"|"google","place_id":"...","name":"...","note":"..."}
func (srv *Server) handleAddTripPlace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req addTripPlaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid JSON body"})
		return
	}
	if req.Source != "local" && req.Source != "google" {
		writeJSON(w, http.StatusBadRequest, errorResponse{`source must be "local" or "google"`})
		return
	}
	if req.PlaceID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"place_id is required"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"name is required"})
		return
	}

	trip, ok := srv.trips.AddPlace(id, req.Source, req.PlaceID, req.Name, req.Note)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{"trip not found"})
		return
	}
	writeJSON(w, http.StatusCreated, trip)
}
