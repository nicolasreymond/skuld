package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 1 cours : mercredi 16.09.2026 10:30-12:00 (Europe/Zurich), hebdo jusqu'au 27.01.2027,
// avec le 21.10.2026 exclu (vacances).
const sampleICS = `BEGIN:VCALENDAR
BEGIN:VEVENT
DTSTART;TZID=Europe/Zurich:20260916T103000
DTEND;TZID=Europe/Zurich:20260916T120000
SUMMARY:MAT3-A-C1
LOCATION:G01
RRULE:FREQ=WEEKLY;INTERVAL=1;UNTIL=20270127T093000Z
EXDATE;TZID=Europe/Zurich:20261021T103000
END:VEVENT
END:VCALENDAR
`

func icsTmp(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "h.ics")
	os.WriteFile(p, []byte(sampleICS), 0o644)
	return p
}

func zurich(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		t.Skip("tz Europe/Zurich indisponible")
	}
	return loc
}

func TestNextClass_thisWeek(t *testing.T) {
	loc := zurich(t)
	// mardi 15.09 08:00 → le prochain est le mercredi 16.09 10:30
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, loc)
	l, err := nextClass(icsTmp(t), now)
	if err != nil || l == nil {
		t.Fatalf("attendu un cours, err=%v", err)
	}
	if l.Summary != "MAT3-A-C1" || l.Location != "G01" {
		t.Fatalf("mauvais cours : %+v", l)
	}
	if !l.Start.Equal(time.Date(2026, 9, 16, 10, 30, 0, 0, loc)) {
		t.Fatalf("mauvais début : %v", l.Start)
	}
}

func TestNextClass_skipsEXDATE(t *testing.T) {
	loc := zurich(t)
	// mercredi 21.10 09:00 : l'occurrence du 21.10 est exclue → prochain = 28.10 10:30
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, loc)
	l, _ := nextClass(icsTmp(t), now)
	if l == nil || !l.Start.Equal(time.Date(2026, 10, 28, 10, 30, 0, 0, loc)) {
		t.Fatalf("EXDATE non respecté : %+v", l)
	}
}

func TestNextClass_afterUNTIL(t *testing.T) {
	loc := zurich(t)
	now := time.Date(2027, 2, 1, 0, 0, 0, 0, loc) // après UNTIL
	l, _ := nextClass(icsTmp(t), now)
	if l != nil {
		t.Fatalf("aucun cours attendu après UNTIL, obtenu %+v", l)
	}
}

func TestNextClass_summaryWithParam(t *testing.T) {
	loc := zurich(t)
	ics := "BEGIN:VCALENDAR\nBEGIN:VEVENT\n" +
		"DTSTART;TZID=Europe/Zurich:20260916T103000\n" +
		"DTEND;TZID=Europe/Zurich:20260916T120000\n" +
		"SUMMARY;LANGUAGE=fr:Cours test\n" +
		"LOCATION;X-PARAM=1:G01\n" +
		"END:VEVENT\nEND:VCALENDAR\n"
	p := filepath.Join(t.TempDir(), "p.ics")
	os.WriteFile(p, []byte(ics), 0o644)
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, loc)
	l, err := nextClass(p, now)
	if err != nil || l == nil || l.Summary != "Cours test" || l.Location != "G01" {
		t.Fatalf("SUMMARY/LOCATION avec paramètre mal parsés : %+v (err=%v)", l, err)
	}
}
