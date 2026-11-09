package web

import (
	"net/http"
	"strconv"

	"github.com/rydncn/workspots/internal/spots"
)

// searchForm is the search as typed, kept on the page (AC2, AC4).
type searchForm struct {
	Area                                                  string
	Wifi, Noise                                           string
	Power, NoTimeLimit, StepFree, AccessibleToilet, Quiet bool
}

func (f searchForm) filter() spots.Filter {
	wifi, _ := strconv.Atoi(f.Wifi)
	noise, _ := strconv.Atoi(f.Noise)
	return spots.Filter{Area: f.Area, MinWifi: clamp(wifi, 0, 4), MaxNoise: clamp(noise, 0, 3),
		Power: f.Power, NoTimeLimit: f.NoTimeLimit, StepFree: f.StepFree, AccessibleToilet: f.AccessibleToilet, Quiet: f.Quiet}
}

func clamp(v, lo, hi int) int16 {
	return int16(max(lo, min(hi, v)))
}

func readSearch(r *http.Request) searchForm {
	q := r.URL.Query()
	on := func(name string) bool { return q.Get(name) == "1" }
	area := q.Get("area")
	if len(area) > 80 {
		area = area[:80]
	}
	return searchForm{Area: area, Wifi: q.Get("wifi"), Noise: q.Get("noise"), Power: on("power"),
		NoTimeLimit: on("no_time_limit"), StepFree: on("step_free"), AccessibleToilet: on("accessible_toilet"), Quiet: on("quiet_space")}
}

func (s *server) search(w http.ResponseWriter, r *http.Request) {
	form := readSearch(r)
	list, err := spots.Search(r.Context(), s.db, form.filter())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "search.html", map[string]any{
		"Form": form, "Spots": list, "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels})
}

func (s *server) spot(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, spots.ErrSpotNotFound)
		return
	}
	spot, err := spots.Get(r.Context(), s.db, id)
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
