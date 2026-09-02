# Place Ratings — POC

A minimal working app: rate a place, search for a place, see its average
rating and every comment left on it.

## Structure

```
backend/    Go API (standard library only, no external dependencies)
frontend/   Next.js app (TypeScript, Tailwind, App Router)
```

## Backend — Go

No third-party packages required, so `go run` / `go build` works with no
network access needed.

```bash
cd backend
go run .
# Server listens on http://localhost:8080
```

Seeded with two example places (Taj Mahal, Gateway of India) so search
returns something immediately.

### Endpoints

| Method | Path                  | Description                                  |
|--------|-----------------------|-----------------------------------------------|
| GET    | /api/health            | Health check                                  |
| POST   | /api/places/review     | Add a review — creates the place if new       |
| GET    | /api/places/search?q=  | Search places by name (empty q = list all)    |

**POST /api/places/review** body:
```json
{ "name": "Taj Mahal", "rating": 5, "comment": "Stunning at sunrise." }
```

**GET /api/places/search?q=taj** response:
```json
[
  {
    "id": "place_...",
    "name": "Taj Mahal",
    "average_rating": 4.7,
    "review_count": 3,
    "reviews": [ { "id": "...", "rating": 5, "comment": "...", "created_at": "..." } ]
  }
]
```

Data is in-memory only — it resets when the backend restarts. That's
intentional for a POC; swap `Store` for a real database later without
touching the HTTP layer.

## Frontend — Next.js

```bash
cd frontend
npm install          # already run once; re-run if you clean node_modules
npm run dev           # dev server on http://localhost:3000
# or, production build (already generated in this delivery):
npm run build
npm run start
```

Set `NEXT_PUBLIC_API_URL` in `.env.local` if the backend isn't on
`http://localhost:8080` (already set to that default).

## Running both together

```bash
# terminal 1
cd backend && go run .

# terminal 2
cd frontend && npm run dev
```

Open http://localhost:3000, submit a review, then search for the place
name to see the average rating and comment list update.

## Google Places (live fetch layer)

This adds a second, separate search that pulls **live** ratings/reviews
straight from Google — nothing from Google is stored in our database.
That's not a design choice, it's a requirement: Google's Places API
policy prohibits caching/storing content (ratings, review text, names,
photos) beyond narrow exceptions (place_id forever, lat/lng for 30 days).
See: https://developers.google.com/maps/documentation/places/web-service/policies

### Setup

1. Create a Google Cloud project, enable billing, enable **"Places API
   (New)"** (not the legacy Places API — different product/pricing),
   create an API key, and restrict it to Places API (New).
2. Set the key as an environment variable before starting the backend:
   ```bash
   export GOOGLE_PLACES_API_KEY="your-key-here"
   cd backend && go run .
   ```
   If this isn't set, the backend still starts fine — the Google
   endpoints just return a clear "not configured" error, and the rest of
   the app (your own reviews) works as normal.

### New endpoints

| Method | Path                              | Description                                    |
|--------|------------------------------------|-------------------------------------------------|
| GET    | /api/google/search?q=              | Live text search against Google Places (New)    |
| GET    | /api/google/place?place_id=        | Live details + reviews for a specific place_id  |

### What's shown

- Rating, rating count, open-now status, address
- Individual reviews with author name/photo (linked to their Google
  profile) and relative time ("3 weeks ago")
- Required attribution line + a "View on Google Maps" link

### Known limitation in this delivery

This code was written and compile-checked against Google's documented
API, but **not live-tested against Google's actual servers** — the
sandbox this was built in doesn't have network access to
`googleapis.com`. Test it on your machine once your key is set up; if
anything in the response shape doesn't match (Google does update field
names occasionally), the parsing lives in `backend/google_places.go` and
should be a small, isolated fix.

## Trips (Phase 1: the retention loop)

This is the feature that gives someone a reason to come back to the app —
save reviewed places (yours or Google's) into a named trip, then view
that trip as an ordered list.

### New endpoints

| Method | Path                          | Description                                      |
|--------|--------------------------------|---------------------------------------------------|
| POST   | /api/trips                     | Create a trip: `{"name": "Goa Dec Trip"}`          |
| GET    | /api/trips                     | List all trips (id, name, place count) — "My Trips"|
| GET    | /api/trips/{id}                | Full trip detail with all places                  |
| POST   | /api/trips/{id}/places         | Add a place to a trip (local or Google source)     |

**POST /api/trips/{id}/places** body:
```json
{ "source": "local", "place_id": "place_xyz", "name": "Baga Beach", "note": "Sunset spot" }
```
`source` is `"local"` (your own reviewed place) or `"google"` (a Google
Places result). `place_id` is either our own place id or Google's
place_id — the name is denormalized at add-time so the trip still
displays correctly without extra fetches.

### New frontend pages

- `/trips` — list of trips + create-new-trip form
- `/trips/[id]` — a single trip's places, in the order added
- "+ Add to trip" button now appears on every place in both the
  community search results and the live Google results — pick an
  existing trip or type a new name inline, no page navigation needed

No auth yet, so "My Trips" currently means "all trips anyone created" —
matches the rest of the POC's no-login scope. Swapping in real user
accounts later is a matter of adding a `user_id` to `Trip` and filtering
the list/create calls — the shape doesn't need to change.

## Out of scope (by design, per the POC requirements)

- Auth/accounts, editing/deleting reviews, photos, maps, moderation,
  external data sources. See the requirements doc for the full list.
