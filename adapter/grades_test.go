package main

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleGrades = `{
  "ASD": {
    "L1": {"course":"ASD","type":"Laboratoire","description":"L1","date":"2026-02-16T00:00:00Z","weight":100,"grade":"5.5","classMean":"5.2"},
    "L2": {"course":"ASD","type":"Laboratoire","description":"L2","date":"2026-03-09T00:00:00Z","weight":100,"grade":"5.9","classMean":"5.6"}
  },
  "MAT": {
    "T1": {"course":"MAT","type":"Test","description":"T1","date":"2026-03-01T00:00:00Z","weight":50,"grade":"4.0","classMean":"4.5"},
    "T2": {"course":"MAT","type":"Test","description":"T2","date":"2026-04-01T00:00:00Z","weight":150,"grade":"5.0","classMean":"4.8"}
  }
}`

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "grades.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadGrades(t *testing.T) {
	g, err := loadGrades(writeTmp(t, sampleGrades))
	if err != nil {
		t.Fatal(err)
	}
	if len(g) != 4 {
		t.Fatalf("attendu 4 notes, obtenu %d", len(g))
	}
}

func TestCourseAverages_weighted(t *testing.T) {
	g, _ := loadGrades(writeTmp(t, sampleGrades))
	avg := courseAverages(g)
	// ASD : (5.5*100 + 5.9*100)/200 = 5.7
	if v := avg["ASD"]; v < 5.69 || v > 5.71 {
		t.Fatalf("ASD attendu 5.70, obtenu %.3f", v)
	}
	// MAT : (4.0*50 + 5.0*150)/200 = 4.75
	if v := avg["MAT"]; v < 4.74 || v > 4.76 {
		t.Fatalf("MAT attendu 4.75, obtenu %.3f", v)
	}
}

func TestOverallAverage(t *testing.T) {
	g, _ := loadGrades(writeTmp(t, sampleGrades))
	o := overallAverage(courseAverages(g))
	// (5.70 + 4.75)/2 = 5.225
	if o < 5.22 || o > 5.23 {
		t.Fatalf("overall attendu 5.225, obtenu %.3f", o)
	}
}

func TestSemesterOf(t *testing.T) {
	cases := map[string]string{
		"2025-09-10T00:00:00Z": "Automne 2025",
		"2025-12-01T00:00:00Z": "Automne 2025",
		"2026-01-13T00:00:00Z": "Automne 2025", // janvier → automne précédent
		"2026-02-16T00:00:00Z": "Printemps 2026",
		"2026-06-04T00:00:00Z": "Printemps 2026",
	}
	for d, want := range cases {
		if got := semesterOf(d); got != want {
			t.Fatalf("semesterOf(%s)=%q, attendu %q", d, got, want)
		}
	}
}

func TestSemesterAverages(t *testing.T) {
	g, _ := loadGrades(writeTmp(t, sampleGrades))
	// sampleGrades : ASD (fév/mars 2026) + MAT (mars/avr 2026) → tous Printemps 2026.
	sems := semesterAverages(g)
	if len(sems) != 1 {
		t.Fatalf("attendu 1 semestre, obtenu %d : %v", len(sems), sems)
	}
	pr, ok := sems["Printemps 2026"]
	if !ok {
		t.Fatalf("semestre « Printemps 2026 » absent : %v", sems)
	}
	if v := pr.Courses["ASD"]; v < 5.69 || v > 5.71 {
		t.Fatalf("ASD Printemps 2026 attendu 5.70, obtenu %.3f", v)
	}
	if pr.Overall < 5.22 || pr.Overall > 5.23 { // (5.70+4.75)/2
		t.Fatalf("overall Printemps 2026 attendu 5.225, obtenu %.3f", pr.Overall)
	}
}
