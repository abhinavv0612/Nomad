package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---------- Data model ----------

type Review struct {
	ID        string    `json:"id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type Place struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Reviews []*Review `json:"reviews"`
}

type PlaceResult struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	AverageRating float64   `json:"average_rating"`
	ReviewCount   int       `json:"review_count"`
	Reviews       []*Review `json:"reviews"`
}

// ---------- In-memory store ----------

type Store struct {
	mu     sync.RWMutex
	places map[string]*Place // keyed by normalized (lowercased, trimmed) name
}

func NewStore() *Store {
	return &Store{places: make(map[string]*Place)}
}

func normalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func genID(prefix string) string {
	return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000")
}

// AddReview creates the place if it doesn't exist, then appends a review.
func (s *Store) AddReview(name string, rating int, comment string) *Place {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalize(name)
	p, ok := s.places[key]
	if !ok {
		p = &Place{
			ID:      genID("place"),
			Name:    strings.TrimSpace(name),
			Reviews: []*Review{},
		}
		s.places[key] = p
	}

	p.Reviews = append(p.Reviews, &Review{
		ID:        genID("review"),
		Rating:    rating,
		Comment:   comment,
		CreatedAt: time.Now().UTC(),
	})

	return p
}

// Search returns places whose name contains the query (case-insensitive).
// An empty query returns all places.
func (s *Store) Search(query string) []*PlaceResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := normalize(query)
	results := []*PlaceResult{}

	for _, p := range s.places {
		if q != "" && !strings.Contains(normalize(p.Name), q) {
			continue
		}
		results = append(results, toResult(p))
	}
	return results
}

func toResult(p *Place) *PlaceResult {
	sum := 0
	for _, r := range p.Reviews {
		sum += r.Rating
	}
	avg := 0.0
	if len(p.Reviews) > 0 {
		avg = float64(sum) / float64(len(p.Reviews))
	}
	return &PlaceResult{
		ID:            p.ID,
		Name:          p.Name,
		AverageRating: roundTo1DP(avg),
		ReviewCount:   len(p.Reviews),
		Reviews:       p.Reviews,
	}
}

func roundTo1DP(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ---------- HTTP handlers ----------

type Server struct {
	store        *Store
	trips        *TripStore
	googlePlaces *GooglePlacesClient // nil if GOOGLE_PLACES_API_KEY isn't set
}

type addReviewRequest struct {
	Name    string `json:"name"`
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (srv *Server) handleAddReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"method not allowed"})
		return
	}

	var req addReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{"invalid JSON body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"name is required"})
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		writeJSON(w, http.StatusBadRequest, errorResponse{"rating must be between 1 and 5"})
		return
	}
	if req.Comment == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{"comment is required"})
		return
	}

	place := srv.store.AddReview(req.Name, req.Rating, req.Comment)
	writeJSON(w, http.StatusCreated, toResult(place))
}

func (srv *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{"method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	results := srv.store.Search(query)
	writeJSON(w, http.StatusOK, results)
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- CORS + middleware ----------

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	}
}

// ---------- main ----------

func main() {
	srv := &Server{store: NewStore(), trips: NewTripStore()}

	// Seed a couple of example places so search has something to show immediately.
	srv.store.AddReview("Taj Mahal", 5, "Breathtaking at sunrise, go early to avoid crowds.")
	srv.store.AddReview("Taj Mahal", 4, "Beautiful but very crowded by mid-morning.")
	srv.store.AddReview("Gateway of India", 4, "Great spot in the evening, lots of street food nearby.")

	if gp, err := NewGooglePlacesClient(); err != nil {
		log.Printf("Google Places live-fetch disabled: %v", err)
	} else {
		srv.googlePlaces = gp
		log.Printf("Google Places live-fetch enabled")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", withLogging(withCORS(srv.handleHealth)))
	mux.HandleFunc("/api/places/review", withLogging(withCORS(srv.handleAddReview)))
	mux.HandleFunc("/api/places/search", withLogging(withCORS(srv.handleSearch)))
	mux.HandleFunc("/api/google/search", withLogging(withCORS(srv.handleGoogleSearch)))
	mux.HandleFunc("/api/google/place", withLogging(withCORS(srv.handleGoogleDetails)))
	mux.HandleFunc("POST /api/trips", withLogging(withCORS(srv.handleCreateTrip)))
	mux.HandleFunc("GET /api/trips", withLogging(withCORS(srv.handleListTrips)))
	mux.HandleFunc("GET /api/trips/{id}", withLogging(withCORS(srv.handleGetTrip)))
	mux.HandleFunc("POST /api/trips/{id}/places", withLogging(withCORS(srv.handleAddTripPlace)))
	// OPTIONS preflight for the trip routes (browsers send this before POST with JSON body)
	mux.HandleFunc("OPTIONS /api/trips", withCORS(func(w http.ResponseWriter, r *http.Request) {}))
	mux.HandleFunc("OPTIONS /api/trips/{id}/places", withCORS(func(w http.ResponseWriter, r *http.Request) {}))

	addr := ":8080"
	log.Printf("place-review backend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
