package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// classSuffix : suffixe de classe d'un SUMMARY ICS (ex. "-C1", "-L2").
var classSuffix = regexp.MustCompile(`-[CL]\d+$`)

// enrolledCourses : codes de cours distincts dérivés des SUMMARY de l'ICS
// (ex. "BDR-A-C1" → "BDR-A"). Les entrées sans suffixe de classe (ex.
// "Travail personnel") sont ignorées. L'ICS ne contient que le semestre courant.
func enrolledCourses(icsPath string) []string {
	b, err := os.ReadFile(icsPath)
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "SUMMARY:") {
			continue
		}
		s := afterColon(line)
		if loc := classSuffix.FindStringIndex(s); loc != nil {
			set[s[:loc[0]]] = true
		}
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

type Lesson struct {
	Summary  string    `json:"summary"`
	Location string    `json:"location"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
}

type vevent struct {
	summary, location string
	start, end        time.Time
	until             time.Time
	exdates           map[string]bool // clé = "20261021T103000"
	dur               time.Duration
}

// nextClass : prochain cours ≥ now sur l'ensemble des VEVENT hebdomadaires.
func nextClass(icsPath string, now time.Time) (*Lesson, error) {
	b, err := os.ReadFile(icsPath)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		return nil, err
	}
	events := parseICS(unfold(string(b)), loc)

	var best *Lesson
	for _, ev := range events {
		occ := ev.nextOccurrence(now)
		if occ == nil {
			continue
		}
		if best == nil || occ.Start.Before(best.Start) {
			best = occ
		}
	}
	return best, nil
}

// currentAndNext : le cours en cours (couvrant now) et le prochain, sur un ICS.
func currentAndNext(icsPath string, now time.Time) (current, next *Lesson, err error) {
	b, err := os.ReadFile(icsPath)
	if err != nil {
		return nil, nil, err
	}
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		return nil, nil, err
	}
	events := parseICS(unfold(string(b)), loc)
	for _, ev := range events {
		if occ := ev.occurrenceCovering(now); occ != nil {
			if current == nil || occ.Start.Before(current.Start) {
				current = occ
			}
		}
		if occ := ev.nextOccurrence(now); occ != nil {
			if next == nil || occ.Start.Before(next.Start) {
				next = occ
			}
		}
	}
	return current, next, nil
}

// daySchedule : tous les cours d'un ICS pour un jour calendaire donné, triés.
func daySchedule(icsPath string, day time.Time) ([]Lesson, error) {
	b, err := os.ReadFile(icsPath)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		return nil, err
	}
	events := parseICS(unfold(string(b)), loc)
	var out []Lesson
	for _, ev := range events {
		if occ := ev.occurrenceOnDay(day); occ != nil {
			out = append(out, *occ)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// occurrenceOnDay : l'occurrence hebdo tombant le jour 'day' (même jour de semaine),
// dans les bornes (until, EXDATE), ou nil.
func (ev vevent) occurrenceOnDay(day time.Time) *Lesson {
	if ev.start.Weekday() != day.Weekday() {
		return nil
	}
	s := time.Date(day.Year(), day.Month(), day.Day(),
		ev.start.Hour(), ev.start.Minute(), ev.start.Second(), 0, ev.start.Location())
	if s.Before(ev.start) {
		return nil // avant la première occurrence
	}
	if !ev.until.IsZero() && s.After(ev.until) {
		return nil
	}
	if ev.exdates[s.Format("20060102T150405")] {
		return nil
	}
	return &Lesson{Summary: ev.summary, Location: ev.location, Start: s, End: s.Add(ev.dur)}
}

// occurrenceCovering : l'occurrence hebdo en cours à now (start ≤ now < end), ou nil.
func (ev vevent) occurrenceCovering(now time.Time) *Lesson {
	s := ev.start
	for !s.Add(ev.dur).After(now) { // avance tant que la fin ≤ now
		s = s.AddDate(0, 0, 7)
	}
	if !ev.until.IsZero() && s.After(ev.until) {
		return nil
	}
	if s.After(now) { // pas encore commencé
		return nil
	}
	if ev.exdates[s.Format("20060102T150405")] {
		return nil
	}
	return &Lesson{Summary: ev.summary, Location: ev.location, Start: s, End: s.Add(ev.dur)}
}

// friendName : « Horaire_Perret_Jonatan_S1_2026_2027.ics » → « Jonatan Perret ».
func friendName(file string) string {
	base := strings.TrimSuffix(filepath.Base(file), ".ics")
	base = strings.TrimPrefix(base, "Horaire_")
	semTok := regexp.MustCompile(`^S\d`)
	var toks []string
	for _, t := range strings.Split(base, "_") {
		if semTok.MatchString(t) {
			break
		}
		toks = append(toks, t)
	}
	if len(toks) == 2 {
		return toks[1] + " " + toks[0] // Prénom Nom
	}
	return strings.Join(toks, " ")
}

// nextOccurrence : la prochaine occurrence hebdo ≥ now (hors EXDATE, ≤ until).
func (ev vevent) nextOccurrence(now time.Time) *Lesson {
	s := ev.start
	// avancer par pas de 7 jours jusqu'à atteindre ou dépasser now
	for s.Before(now) {
		s = s.AddDate(0, 0, 7)
	}
	for {
		if !ev.until.IsZero() && s.After(ev.until) {
			return nil
		}
		key := s.Format("20060102T150405")
		if !ev.exdates[key] {
			return &Lesson{Summary: ev.summary, Location: ev.location, Start: s, End: s.Add(ev.dur)}
		}
		s = s.AddDate(0, 0, 7) // occurrence exclue → semaine suivante
	}
}

// unfold : déplie les lignes ICS (RFC 5545 : une ligne commençant par espace continue la précédente).
func unfold(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\n ", "")
}

func parseICS(s string, loc *time.Location) []vevent {
	var out []vevent
	for _, block := range strings.Split(s, "BEGIN:VEVENT") {
		if !strings.Contains(block, "END:VEVENT") {
			continue
		}
		ev := vevent{exdates: map[string]bool{}}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "SUMMARY"):
				ev.summary = afterColon(line)
			case strings.HasPrefix(line, "LOCATION"):
				ev.location = afterColon(line)
			case strings.HasPrefix(line, "DTSTART"):
				ev.start = parseDT(afterColon(line), loc)
			case strings.HasPrefix(line, "DTEND"):
				ev.end = parseDT(afterColon(line), loc)
			case strings.HasPrefix(line, "RRULE:"):
				if u := valueOf(line, "UNTIL="); u != "" {
					ev.until = parseDT(u, loc)
				}
			case strings.HasPrefix(line, "EXDATE"):
				for _, d := range strings.Split(afterColon(line), ",") {
					d = strings.TrimSpace(d)
					if d != "" {
						ev.exdates[normDT(d)] = true
					}
				}
			}
		}
		if !ev.start.IsZero() {
			if !ev.end.IsZero() {
				ev.dur = ev.end.Sub(ev.start)
			}
			out = append(out, ev)
		}
	}
	return out
}

func afterColon(line string) string {
	if i := strings.LastIndex(line, ":"); i >= 0 {
		return strings.TrimSpace(line[i+1:])
	}
	return ""
}

func valueOf(line, key string) string {
	i := strings.Index(line, key)
	if i < 0 {
		return ""
	}
	rest := line[i+len(key):]
	if j := strings.IndexAny(rest, ";\n"); j >= 0 {
		return rest[:j]
	}
	return strings.TrimSpace(rest)
}

// parseDT : "20260916T103000" (local) ou "20270127T093000Z" (UTC) → time.Time.
func parseDT(v string, loc *time.Location) time.Time {
	v = strings.TrimSpace(v)
	if strings.HasSuffix(v, "Z") {
		t, _ := time.Parse("20060102T150405Z", v)
		return t.In(loc)
	}
	t, _ := time.ParseInLocation("20060102T150405", v, loc)
	return t
}

// normDT : clé de comparaison EXDATE (on retire un éventuel suffixe Z).
func normDT(v string) string { return strings.TrimSuffix(strings.TrimSpace(v), "Z") }
