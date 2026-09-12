// Adaptateur gaps-cli → ntfy.
// gaps-cli (scraper) POST vers {api-url}/api/grade un JSON {course,class,name,class_average}
// avec Authorization: Bearer <ADAPTER_API_KEY>. On le republie en notification ntfy.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type apiGrade struct {
	Course string  `json:"course"`
	Class  string  `json:"class"`
	Name   string  `json:"name"`
	Mean   float32 `json:"class_average"`
}

func main() {
	apiKey := os.Getenv("ADAPTER_API_KEY")
	ntfyURL := envOr("NTFY_URL", "https://ntfy.sh") // racine ntfy (publication JSON)
	ntfyTopic := envOr("NTFY_TOPIC", "skuld")
	ntfyToken := os.Getenv("NTFY_TOKEN")
	addr := envOr("LISTEN", ":8080")
	gradesFile := envOr("GRADES_FILE", "/history/grades.json")
	icsFile := envOr("ICS_FILE", "/data/horaire.ics")
	friendsDir := envOr("FRIENDS_DIR", "/data/friends")

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok") })

	http.HandleFunc("/api/grade", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeErr(w, 405, "method not allowed")
			return
		}
		if apiKey != "" && r.Header.Get("Authorization") != "Bearer "+apiKey {
			writeErr(w, 401, "unauthorized")
			return
		}
		var g apiGrade
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			writeErr(w, 400, "invalid json")
			return
		}

		title := "Nouvelle note — " + strings.TrimSpace(g.Course)
		msg := fmt.Sprintf("%s · %s\nMoyenne de classe : %.2f",
			strings.TrimSpace(g.Class), strings.TrimSpace(g.Name), g.Mean)

		if err := publishNtfy(ntfyURL, ntfyTopic, ntfyToken, title, msg, []string{"mortar_board"}, 4); err != nil {
			log.Printf("ntfy publish error: %v", err)
			writeErr(w, 502, "ntfy error")
			return
		}
		log.Printf("notified: %s / %s (mean %.2f)", g.Course, g.Name, g.Mean)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	// READ_API_KEY protège les endpoints de lecture pour une exposition publique
	// (ex. via Cloudflare). Vide = accès ouvert (mode tailnet privé, rétro-compatible).
	readKey := os.Getenv("READ_API_KEY")
	onlyGet := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				writeErr(w, 405, "method not allowed")
				return
			}
			if readKey != "" && r.Header.Get("Authorization") != "Bearer "+readKey {
				writeErr(w, 401, "unauthorized")
				return
			}
			h(w, r)
		}
	}

	http.HandleFunc("/grades", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		g, err := loadGrades(gradesFile)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, g)
	}))
	http.HandleFunc("/averages", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		g, err := loadGrades(gradesFile)
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]any{"semesters": semesterAverages(g)})
	}))
	http.HandleFunc("/next-class", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		l, err := nextClass(icsFile, time.Now())
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, l)
	}))
	http.HandleFunc("/schedule", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		l, err := nextClass(icsFile, time.Now())
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		writeJSON(w, l)
	}))
	http.HandleFunc("/widget", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		g, _ := loadGrades(gradesFile)
		l, _ := nextClass(icsFile, time.Now())
		var latest *Grade
		var semester string
		var overall float64
		if len(g) > 0 {
			latest = &g[len(g)-1]
			semester = semesterOf(latest.Date)          // semestre de la dernière note
			overall = semesterAverages(g)[semester].Overall
		}
		writeJSON(w, map[string]any{"next_class": l, "semester": semester, "overall": overall, "latest": latest})
	}))

	// /semester : cours du semestre courant (calendaire), affichés même sans note.
	// Union des cours inscrits (ICS) et des cours notés ce semestre ; chacun porte
	// ses notes (éventuellement vides) + sa moyenne (nil si aucune note).
	http.HandleFunc("/semester", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		sem := semesterOf(time.Now().Format(time.RFC3339))
		g, _ := loadGrades(gradesFile)
		byCourse := map[string][]Grade{}
		for _, x := range g {
			if semesterOf(x.Date) == sem {
				byCourse[x.Course] = append(byCourse[x.Course], x)
			}
		}
		set := map[string]bool{}
		for _, c := range enrolledCourses(icsFile) {
			set[c] = true
		}
		for c := range byCourse {
			set[c] = true
		}
		names := make([]string, 0, len(set))
		for c := range set {
			names = append(names, c)
		}
		sort.Strings(names)

		type courseNotes struct {
			Course  string   `json:"course"`
			Average *float64 `json:"average"`
			Grades  []Grade  `json:"grades"`
		}
		courses := make([]courseNotes, 0, len(names))
		for _, c := range names {
			gs := byCourse[c]
			var avg *float64
			if len(gs) > 0 {
				if a, ok := courseAverages(gs)[c]; ok {
					avg = &a
				}
			}
			if gs == nil {
				gs = []Grade{}
			}
			courses = append(courses, courseNotes{Course: c, Average: avg, Grades: gs})
		}
		writeJSON(w, map[string]any{"semester": sem, "courses": courses})
	}))

	// /target?course=X&goal=G[&weight=W] : note à obtenir sur une prochaine épreuve
	// (de poids W, par défaut le poids moyen des épreuves du cours) pour atteindre la
	// moyenne visée G. Modèle : moyenne pondérée par le poids des notes.
	http.HandleFunc("/target", onlyGet(func(w http.ResponseWriter, r *http.Request) {
		course := r.URL.Query().Get("course")
		goalStr := r.URL.Query().Get("goal")
		if course == "" || goalStr == "" {
			writeErr(w, 400, "paramètres 'course' et 'goal' requis")
			return
		}
		goal, err := strconv.ParseFloat(goalStr, 64)
		if err != nil {
			writeErr(w, 400, "'goal' invalide")
			return
		}
		g, _ := loadGrades(gradesFile)
		var sumW, sumWG float64
		var count int
		for _, x := range g {
			if x.Course != course {
				continue
			}
			v, e := strconv.ParseFloat(x.Grade, 64)
			if e != nil {
				continue
			}
			sumW += x.Weight
			sumWG += v * x.Weight
			count++
		}
		if sumW == 0 {
			writeErr(w, 404, "cours inconnu ou sans note")
			return
		}
		current := sumWG / sumW
		weight := sumW / float64(count) // poids moyen d'une épreuve
		if ws := r.URL.Query().Get("weight"); ws != "" {
			if wv, e := strconv.ParseFloat(ws, 64); e == nil && wv > 0 {
				weight = wv
			}
		}
		needed := (goal*(sumW+weight) - sumWG) / weight
		writeJSON(w, map[string]any{
			"course":         course,
			"current":        current,
			"goal":           goal,
			"assumed_weight": weight,
			"needed":         needed,          // note à obtenir sur la prochaine épreuve
			"achievable":     needed <= 6.0,   // barème /6
			"already":        current >= goal, // objectif déjà atteint
		})
	}))

	// /friends : statut de chaque ami (cours en cours + prochain cours), depuis les
	// ICS déposés dans FRIENDS_DIR.
	http.HandleFunc("/friends", onlyGet(func(w http.ResponseWriter, _ *http.Request) {
		entries, _ := os.ReadDir(friendsDir)
		now := time.Now()
		type friendStatus struct {
			Name    string  `json:"name"`
			InClass bool    `json:"in_class"`
			Current *Lesson `json:"current"`
			Next    *Lesson `json:"next"`
		}
		out := make([]friendStatus, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".ics") {
				continue
			}
			cur, nxt, err := currentAndNext(filepath.Join(friendsDir, e.Name()), now)
			if err != nil {
				continue
			}
			out = append(out, friendStatus{Name: friendName(e.Name()), InClass: cur != nil, Current: cur, Next: nxt})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		writeJSON(w, map[string]any{"friends": out})
	}))

	// Rappel « cours dans N min » (0 = désactivé).
	if mins, err := strconv.Atoi(envOr("REMINDER_MINUTES", "15")); err == nil && mins > 0 {
		go reminderLoop(icsFile, ntfyURL, ntfyTopic, ntfyToken, time.Duration(mins)*time.Minute)
		log.Printf("rappel cours activé : %d min avant", mins)
	}

	log.Printf("gaps→ntfy adapter listening on %s (topic=%s)", addr, ntfyTopic)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"code": code, "message": msg})
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
