package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rydncn/workspots/internal/moderation"
	"github.com/rydncn/workspots/internal/spots"
)

// Form field limits, matching the database constraints in migrations 0001
// and 0002 so a valid form never hits one.
const (
	maxName, maxArea, maxAddress, maxHours, maxNote, maxReason = 120, 80, 200, 500, 500, 500
)

// Contributions per member in a quarter hour, against flooding. (One account
// cannot set a spot's attributes however many reports it posts: the
// spot_attributes view counts one vote per member, migration 0005.)
const (
	spotsPerMember, reportsPerMember, problemsPerMember = 10, 20, 20
)

// limitMember answers 429 and returns false when the member has used up kind.
func (s *server) limitMember(w http.ResponseWriter, r *http.Request, kind string, userID int64, limit int) bool {
	if wait := s.limits.take(kind+":user:"+strconv.FormatInt(userID, 10), limit); wait > 0 {
		s.rateLimited(w, r, wait)
		return false
	}
	return true
}

// field reads a form value, trimmed and without the bytes Postgres refuses (NUL,
// invalid UTF-8), as readSearch does for the query string.
func field(r *http.Request, name string) string {
	return strings.TrimSpace(strings.ToValidUTF8(strings.ReplaceAll(r.PostForm.Get(name), "\x00", ""), ""))
}

func tooLong(s string, n int) bool { return utf8.RuneCountInString(s) > n }

// readAttributes reads the rating fields, reporting any out of range.
func readAttributes(r *http.Request) (spots.Attributes, []fieldError) {
	var a spots.Attributes
	var errs []fieldError
	wifi, err := strconv.Atoi(r.PostForm.Get("wifi"))
	if err != nil || wifi < 0 || wifi > 4 {
		errs = append(errs, fieldError{"wifi", "Choose the Wi-Fi quality."})
	}
	noise, err := strconv.Atoi(r.PostForm.Get("noise"))
	if err != nil || noise < 1 || noise > 3 {
		errs = append(errs, fieldError{"noise", "Choose the noise level."})
	}
	a.Wifi, a.Noise = int16(wifi), int16(noise)
	on := func(name string) bool { return r.PostForm.Get(name) == "1" }
	a.Power, a.TimeLimit, a.StepFree, a.AccessibleToilet, a.Quiet = on("power"), on("time_limit"), on("step_free"), on("accessible_toilet"), on("quiet_space")
	return a, errs
}

func (s *server) spotFormData(n spots.NewSpot) map[string]any {
	return map[string]any{"Spot": n, "Kinds": spots.Kinds, "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels}
}

func (s *server) newSpotForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	s.render(w, r, http.StatusOK, "spot-new.html", s.spotFormData(spots.NewSpot{Attributes: spots.Attributes{Wifi: 2, Noise: 2}}))
}

func (s *server) newSpotPost(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	n := spots.NewSpot{Name: field(r, "name"), Kind: field(r, "kind"), Area: field(r, "area"),
		Address: field(r, "address"), OpeningHours: field(r, "opening_hours")}
	var errs []fieldError
	a, aerrs := readAttributes(r)
	n.Attributes = a
	errs = append(errs, aerrs...)
	switch {
	case n.Name == "":
		errs = append(errs, fieldError{"name", "Give the spot a name."})
	case tooLong(n.Name, maxName):
		errs = append(errs, fieldError{"name", "The name must be at most 120 characters."})
	}
	switch {
	case n.Area == "":
		errs = append(errs, fieldError{"area", "Say which town or postcode district it is in."})
	case tooLong(n.Area, maxArea):
		errs = append(errs, fieldError{"area", "The area must be at most 80 characters."})
	}
	switch {
	case n.Address == "":
		errs = append(errs, fieldError{"address", "Give the address."})
	case tooLong(n.Address, maxAddress):
		errs = append(errs, fieldError{"address", "The address must be at most 200 characters."})
	}
	if tooLong(n.OpeningHours, maxHours) {
		errs = append(errs, fieldError{"opening_hours", "Opening hours must be at most 500 characters."})
	}
	if !contains(spots.Kinds, n.Kind) {
		errs = append(errs, fieldError{"kind", "Choose what kind of place it is."})
	}
	if len(errs) > 0 {
		s.invalid(w, r, "spot-new.html", s.spotFormData(n), errs)
		return
	}
	if !s.limitMember(w, r, "spots", u.ID, spotsPerMember) {
		return
	}
	id, err := spots.Add(r.Context(), s.db, u.ID, n)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/spots/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// publicSpotForForm loads the spot a report is about. Only approved spots can
// be reported on, so the anonymous view is the right one even for the author.
func (s *server) publicSpotForForm(w http.ResponseWriter, r *http.Request) (spots.Spot, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		s.fail(w, r, spots.ErrSpotNotFound)
		return spots.Spot{}, false
	}
	spot, err := spots.Get(r.Context(), s.db, spots.Anonymous, id)
	if err != nil {
		s.fail(w, r, err)
		return spots.Spot{}, false
	}
	return spot, true
}

func (s *server) newReportForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	spot, ok := s.publicSpotForForm(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "report-new.html", map[string]any{"Spot": spot, "Report": spots.Attributes{Wifi: 2, Noise: 2}, "Note": "", "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels})
}

func (s *server) newReportPost(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	spot, ok := s.publicSpotForForm(w, r)
	if !ok {
		return
	}
	a, errs := readAttributes(r)
	note := field(r, "note")
	if tooLong(note, maxNote) {
		errs = append(errs, fieldError{"note", "The note must be at most 500 characters."})
	}
	if len(errs) > 0 {
		s.invalid(w, r, "report-new.html", map[string]any{"Spot": spot, "Report": a, "Note": note, "WifiLabels": spots.WifiLabels, "NoiseLabels": spots.NoiseLabels}, errs)
		return
	}
	if !s.limitMember(w, r, "reports", u.ID, reportsPerMember) {
		return
	}
	if err := spots.AddReport(r.Context(), s.db, u.ID, spot.ID, a, note); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/spots/"+strconv.FormatInt(spot.ID, 10), http.StatusSeeOther)
}

// problem forms: /spots/{id}/problems/new and /reports/{id}/problems/new share a page.
func (s *server) newProblemForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	about, _, ok := s.problemTarget(w, r)
	if !ok {
		return
	}
	s.render(w, r, http.StatusOK, "problem-new.html", map[string]any{"About": about, "Reason": "", "Action": r.URL.Path})
}

func (s *server) newProblemPost(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	about, file, ok := s.problemTarget(w, r)
	if !ok {
		return
	}
	reason := field(r, "reason")
	var errs []fieldError
	if reason == "" {
		errs = append(errs, fieldError{"reason", "Say what the problem is."})
	} else if tooLong(reason, maxReason) {
		errs = append(errs, fieldError{"reason", "The reason must be at most 500 characters."})
	}
	if len(errs) > 0 {
		s.invalid(w, r, "problem-new.html", map[string]any{"About": about, "Reason": reason, "Action": r.URL.Path}, errs)
		return
	}
	if !s.limitMember(w, r, "problems", u.ID, problemsPerMember) {
		return
	}
	if err := file(u.ID, reason); err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "problem-thanks.html", map[string]any{"About": about})
}

// problemTarget works out what the problem form is about from the route, and
// returns the function that files it. An unknown target is a 404 problem.
func (s *server) problemTarget(w http.ResponseWriter, r *http.Request) (about string, file func(int64, string) error, ok bool) {
	id, valid := pathID(r, "id")
	if !valid {
		s.fail(w, r, moderation.ErrTargetNotFound)
		return "", nil, false
	}
	if strings.HasPrefix(r.URL.Path, "/spots/") {
		spot, err := spots.Get(r.Context(), s.db, spots.Anonymous, id) // only public spots can be flagged
		if errors.Is(err, spots.ErrSpotNotFound) {
			err = moderation.ErrTargetNotFound
		}
		if err != nil {
			s.fail(w, r, err)
			return "", nil, false
		}
		return "the spot " + spot.Name, func(uid int64, reason string) error {
			return moderation.ReportSpotProblem(r.Context(), s.db, uid, id, reason)
		}, true
	}
	visible, err := spots.ReportVisible(r.Context(), s.db, id)
	if err == nil && !visible {
		err = moderation.ErrTargetNotFound
	}
	if err != nil {
		s.fail(w, r, err)
		return "", nil, false
	}
	return "a report", func(uid int64, reason string) error {
		return moderation.ReportReportProblem(r.Context(), s.db, uid, id, reason)
	}, true
}
