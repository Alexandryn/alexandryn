package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)


type wireCollectionRef struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	AddedAt *time.Time `json:"addedAt,omitempty"`
}

type wireWorkSummary struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Subtitle    string              `json:"subtitle"`
	Authors     []string            `json:"authors"`
	IsOwned     bool                `json:"isOwned"`
	Collections []wireCollectionRef `json:"collections"`
	AddedAt     *time.Time          `json:"addedAt"`
}

type wireLibraryPage struct {
	Works      []wireWorkSummary `json:"works"`
	NextCursor *string           `json:"nextCursor"`
}

type wireOwnedEdition struct {
	ID              string     `json:"id"`
	Language        string     `json:"language"`
	ISBN            *string    `json:"isbn"`
	Publisher       string     `json:"publisher"`
	PublicationYear *int       `json:"publicationYear"`
	AddedAt         *time.Time `json:"addedAt,omitempty"`
	Formats         []string   `json:"formats"`
}

type wireWorkDetail struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	Subtitle         string              `json:"subtitle"`
	Authors          []string            `json:"authors"`
	Subjects         []string            `json:"subjects,omitempty"`
	OriginalLanguage *string             `json:"originalLanguage"`
	OwnedEditions    []wireOwnedEdition  `json:"ownedEditions"`
	Collections      []wireCollectionRef `json:"collections"`
}

// LibraryHandler returns the HTTP handler for GET /api/v1/library (backend-library-api.md FR-1–FR-4).
func LibraryHandler(repo domain.WorkRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		// Constitution §4: line-1 validation of all query parameters.
		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 || parsedLimit > 100 {
				WriteError(w, domain.InvalidInput,
					fmt.Sprintf("limit: invalid value %q — must be an integer between 1 and 100", limitStr), id)
				return
			}
			limit = parsedLimit
		}

		q := r.URL.Query().Get("q")
		if len([]rune(q)) > 200 {
			WriteError(w, domain.InvalidInput, "q: exceeds maximum length of 200 characters", id)
			return
		}

		filterStr := r.URL.Query().Get("filter")
		filter := domain.FilterAll
		if filterStr != "" {
			switch domain.LibraryFilter(filterStr) {
			case domain.FilterOwned, domain.FilterWanted, domain.FilterAll:
				filter = domain.LibraryFilter(filterStr)
			default:
				WriteError(w, domain.InvalidInput,
					fmt.Sprintf("filter: unrecognized value %q — must be one of: owned, wanted, all", filterStr), id)
				return
			}
		}

		sortStr := r.URL.Query().Get("sort")
		sort := domain.SortAddedAt
		if sortStr != "" {
			switch domain.LibrarySort(sortStr) {
			case domain.SortAddedAt, domain.SortTitle:
				sort = domain.LibrarySort(sortStr)
			default:
				WriteError(w, domain.InvalidInput,
					fmt.Sprintf("sort: unrecognized value %q — must be one of: added_at, title", sortStr), id)
				return
			}
		}

		cursor := r.URL.Query().Get("cursor")
		if cursor != "" {
			if sort == domain.SortTitle {
				if _, _, err := domain.DecodeTitleCursor(cursor); err != nil {
					WriteError(w, domain.InvalidInput, "invalid cursor", id)
					return
				}
			} else {
				if _, _, err := domain.DecodeAddedAtCursor(cursor); err != nil {
					WriteError(w, domain.InvalidInput, "invalid cursor", id)
					return
				}
			}
		}

		page, err := repo.QueryLibrary(r.Context(), domain.LibraryQuery{
			Cursor: cursor,
			Limit:  limit,
			Q:      q,
			Filter: filter,
			Sort:   sort,
		})
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		works := make([]wireWorkSummary, 0, len(page.Works))
		for _, item := range page.Works {
			colls := make([]wireCollectionRef, 0, len(item.Collections))
			for _, c := range item.Collections {
				colls = append(colls, wireCollectionRef{
					ID:      string(c.ID),
					Name:    c.Name,
					AddedAt: c.AddedAt,
				})
			}
			works = append(works, wireWorkSummary{
				ID:          string(item.ID),
				Title:       item.Title,
				Subtitle:    item.Subtitle,
				Authors:     item.Authors,
				IsOwned:     item.IsOwned,
				Collections: colls,
				AddedAt:     item.AddedAt,
			})
		}

		var nextCursor *string
		if page.NextCursor != "" {
			c := page.NextCursor
			nextCursor = &c
		}

		resp := wireLibraryPage{
			Works:      works,
			NextCursor: nextCursor,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// WorkDetailHandler returns the HTTP handler for GET /api/v1/works/{id} (backend-library-api.md FR-5).
func WorkDetailHandler(repo domain.WorkRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		workID := r.PathValue("id")
		if workID == "" && strings.HasPrefix(r.URL.Path, "/api/v1/works/") {
			workID = strings.TrimPrefix(r.URL.Path, "/api/v1/works/")
		}

		// Constitution §4 shape validation:
		if err := domain.ValidateBoundedText("id", workID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid work ID", id)
			return
		}

		detail, err := repo.FindWorkDetail(r.Context(), domain.WorkID(workID))
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				WriteError(w, domain.NotFound, "no work with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		editions := make([]wireOwnedEdition, 0, len(detail.OwnedEditions))
		for _, e := range detail.OwnedEditions {
			formats := e.Formats
			if formats == nil {
				formats = []string{}
			}
			editions = append(editions, wireOwnedEdition{
				ID:              string(e.ID),
				Language:        e.Language,
				ISBN:            e.ISBN,
				Publisher:       e.Publisher,
				PublicationYear: e.PublicationYear,
				AddedAt:         e.AddedAt,
				Formats:         formats,
			})
		}

		colls := make([]wireCollectionRef, 0, len(detail.Collections))
		for _, c := range detail.Collections {
			colls = append(colls, wireCollectionRef{
				ID:      string(c.ID),
				Name:    c.Name,
				AddedAt: c.AddedAt,
			})
		}

		var origLang *string
		if detail.OriginalLanguage != nil {
			s := detail.OriginalLanguage.String()
			origLang = &s
		}

		resp := wireWorkDetail{
			ID:               string(detail.ID),
			Title:            detail.Title,
			Subtitle:         detail.Subtitle,
			Authors:          detail.Authors,
			Subjects:         detail.Subjects,
			OriginalLanguage: origLang,
			OwnedEditions:    editions,
			Collections:      colls,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
