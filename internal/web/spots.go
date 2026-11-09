package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rydncn/workspots/internal/spots"
)

// searchForm is the search as typed, kept on the page (AC2, AC4).
type searchForm struct {
	Area                                                  string
	Wifi, Noise                                           string
	Power, NoTimeLimit, StepFree, AccessibleToilet, Quiet bool
	Page                                                  int
}

func (f searchForm) filter() spots.Filter {
	wifi, _ := strconv.Atoi(f.Wifi)
	noise, _ := strconv.Atoi(f.Noise)
	return spots.Filter{Area: f.Area, MinWifi: clamp(wifi, 0, 4), MaxNoise: clamp(noise, 0, 3),
		Power: f.Power, NoTimeLimit: f.NoTimeLimit, StepFree: f.StepFree, AccessibleToilet: f.AccessibleToilet, Quiet: f.Quiet, Page: f.Page}
}

// pageURL is the search URL for page n with the same filters (no "page=1").
func (f searchForm) pageURL(n int) string {
	q := url.Values{}
	q.Set("area", f.Area)
	for name, on := range map[string]bool{"power": f.Power, "no_time_limit": f.NoTimeLimit, "step_free": f.StepFree, "accessible_toilet": f.AccessibleToilet, "quiet_space": f.Quiet} {
		if on {
			q.Set(name, "1")
		}
	}
	if f.Wifi != "" {
		q.Set("wifi", f.Wifi)
	}
	if f.Noise != "" {
		q.Set("noise", f.Noise)
	}
	q.Set("page", strconv.Itoa(n))
	return "/search?" + q.Encode()
}

func clamp(v, lo, hi int) int16 {
	return int16(max(lo, min(hi, v)))
}

// readSearch cleans the query at the edge: valid UTF-8 without NUL (Postgres
// rejects both), at most 80 characters, filter values clamped to their scales
// so the form echoes what was applied.
func readSearch(r *http.Request) searchForm {
	q := r.URL.Query()
	on := func(name string) bool { return q.Get(name) == "1" }
	area := strings.ToValidUTF8(strings.ReplaceAll(q.Get("area"), "\x00", ""), "")
	if runes := []rune(area); len(runes) > 80 {
		area = string(runes[:80])
	}
	scale := func(name string, hi int) string {
		v, err := strconv.Atoi(q.Get(name))
		if err != nil || v <= 0 {
			return ""
		}
		return strconv.Itoa(min(v, hi))
	}
	page, _ := strconv.Atoi(q.Get("page"))
	return searchForm{Area: area, Wifi: scale("wifi", 4), Noise: scale("noise", 3), Power: on("power"),
		NoTimeLimit: on("no_time_limit"), StepFree: on("step_free"), AccessibleToilet: on("accessible_toilet"), Quiet: on("quiet_space"),
		Page: max(1, min(page, 100000))}
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	form := readSearch(r)
	list, more, err := spots.Search(r.Context(), s.db, form.filter())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var next, prev string
	if more {
		next = form.pageURL(form.Page + 1)
	}
	if form.Page > 1 {
		prev = form.pageURL(form.Page - 1)
	}
	s.render(w, r, http.StatusOK, "search.html", map[string]any{
		"Form": form, "Spots": list, "Next": next, "Prev": prev, "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels})
}

func (s *server) spot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, spots.ErrSpotNotFound)
		return
	}
	spot, err := spots.Get(r.Context(), s.db, viewer(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	reports, err := spots.VisibleReports(r.Context(), s.db, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "spot.html", map[string]any{"Spot": spot, "Reports": reports})
}

// viewer is who is looking, for the visibility rule.
func viewer(r *http.Request) spots.Viewer {
	if u := currentSession(r).user; u != nil {
		return spots.Viewer{UserID: u.ID, Moderator: u.Role == "moderator" || u.Role == "admin"}
	}
	return spots.Anonymous
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}
