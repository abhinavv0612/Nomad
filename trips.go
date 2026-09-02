package main

import (
	"strings"
	"sync"
	"time"
)

// A TripPlace is a place attached to a trip. It can reference either a place
// from our own review store ("local") or a Google Places result ("google").
// We denormalize the name so a trip still displays sensibly even if the
// underlying place is later renamed or, for Google places, if we're not
// re-fetching live details just to render a trip list.
type TripPlace struct {
	ID      string    `json:"id"`
	Source  string    `json:"source"` // "local" or "google"
	PlaceID string    `json:"place_id"`
	Name    string    `json:"name"`
	Note    string    `json:"note,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

type Trip struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	CreatedAt time.Time   `json:"created_at"`
	Places    []TripPlace `json:"places"`
}

type TripSummary struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	PlaceCount int       `json:"place_count"`
}

type TripStore struct {
	mu    sync.RWMutex
	trips map[string]*Trip
	// order preserves creation order so "my trips" lists newest-first
	// without depending on Go's non-deterministic map iteration.
	order []string
}

func NewTripStore() *TripStore {
	return &TripStore{trips: make(map[string]*Trip)}
}

func (s *TripStore) Create(name string) *Trip {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := &Trip{
		ID:        genID("trip"),
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC(),
		Places:    []TripPlace{},
	}
	s.trips[t.ID] = t
	s.order = append([]string{t.ID}, s.order...) // newest first
	return t
}

func (s *TripStore) Get(id string) (*Trip, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.trips[id]
	return t, ok
}

func (s *TripStore) List() []TripSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make([]TripSummary, 0, len(s.order))
	for _, id := range s.order {
		t := s.trips[id]
		summaries = append(summaries, TripSummary{
			ID:         t.ID,
			Name:       t.Name,
			CreatedAt:  t.CreatedAt,
			PlaceCount: len(t.Places),
		})
	}
	return summaries
}

// AddPlace appends a place to a trip. Returns (trip, ok) — ok is false if
// the trip doesn't exist.
func (s *TripStore) AddPlace(tripID, source, placeID, name, note string) (*Trip, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.trips[tripID]
	if !ok {
		return nil, false
	}

	t.Places = append(t.Places, TripPlace{
		ID:      genID("tripplace"),
		Source:  source,
		PlaceID: placeID,
		Name:    strings.TrimSpace(name),
		Note:    strings.TrimSpace(note),
		AddedAt: time.Now().UTC(),
	})
	return t, true
}
