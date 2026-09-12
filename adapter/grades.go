package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

type Grade struct {
	Course      string  `json:"course"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
	Weight      float64 `json:"weight"`
	Grade       string  `json:"grade"`
	ClassMean   string  `json:"classMean"`
}

// loadGrades lit le grades.json du scraper : {cours: {desc: Grade}}.
func loadGrades(path string) ([]Grade, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]Grade
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	var out []Grade
	for _, byDesc := range raw {
		for _, g := range byDesc {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// courseAverages : moyenne pondérée (par Weight) de la note élève, par cours.
// Une note non numérique est ignorée.
func courseAverages(g []Grade) map[string]float64 {
	type acc struct{ sum, w float64 }
	m := map[string]*acc{}
	for _, x := range g {
		v, err := strconv.ParseFloat(x.Grade, 64)
		if err != nil {
			continue
		}
		a := m[x.Course]
		if a == nil {
			a = &acc{}
			m[x.Course] = a
		}
		a.sum += v * x.Weight
		a.w += x.Weight
	}
	out := map[string]float64{}
	for c, a := range m {
		if a.w > 0 {
			out[c] = a.sum / a.w
		}
	}
	return out
}

// overallAverage : moyenne simple des moyennes de cours.
func overallAverage(avgs map[string]float64) float64 {
	if len(avgs) == 0 {
		return 0
	}
	var s float64
	for _, v := range avgs {
		s += v
	}
	return s / float64(len(avgs))
}

// semesterOf : dérive le semestre HEIG d'une date RFC3339.
//   - fév.–juil. → « Printemps <année> »
//   - août–déc.  → « Automne <année> »
//   - janvier    → « Automne <année-1> » (les examens de janvier appartiennent
//     au semestre d'automne qui vient de se terminer).
func semesterOf(dateRFC3339 string) string {
	t, err := time.Parse(time.RFC3339, dateRFC3339)
	if err != nil {
		return "Inconnu"
	}
	y, m := t.Year(), int(t.Month())
	switch {
	case m >= 2 && m <= 7:
		return fmt.Sprintf("Printemps %d", y)
	case m == 1:
		return fmt.Sprintf("Automne %d", y-1)
	default: // août–décembre
		return fmt.Sprintf("Automne %d", y)
	}
}

// SemesterAvg : moyennes par cours + générale, pour un semestre.
type SemesterAvg struct {
	Courses map[string]float64 `json:"courses"`
	Overall float64            `json:"overall"`
}

// semesterAverages : regroupe les notes par semestre HEIG, puis calcule les
// moyennes par cours + la moyenne générale au sein de chaque semestre.
func semesterAverages(g []Grade) map[string]SemesterAvg {
	bySem := map[string][]Grade{}
	for _, x := range g {
		s := semesterOf(x.Date)
		bySem[s] = append(bySem[s], x)
	}
	out := map[string]SemesterAvg{}
	for s, gs := range bySem {
		c := courseAverages(gs)
		out[s] = SemesterAvg{Courses: c, Overall: overallAverage(c)}
	}
	return out
}
