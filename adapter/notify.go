package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// publishNtfy publie une notification sur un topic ntfy (API JSON).
func publishNtfy(ntfyURL, topic, token, title, msg string, tags []string, priority int) error {
	payload, _ := json.Marshal(map[string]any{
		"topic":    topic,
		"title":    title,
		"message":  msg,
		"tags":     tags,
		"priority": priority,
	})
	req, _ := http.NewRequest(http.MethodPost, ntfyURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("ntfy status %d", res.StatusCode)
	}
	return nil
}

// reminderLoop notifie « cours dans N min » quand le prochain cours approche.
// Tourne en tâche de fond, une notification par cours (dédup par clé).
func reminderLoop(icsFile, ntfyURL, topic, token string, lead time.Duration) {
	var lastKey string
	mins := int(lead.Minutes())
	for {
		now := time.Now()
		if l, err := nextClass(icsFile, now); err == nil && l != nil {
			d := l.Start.Sub(now)
			key := l.Summary + "|" + l.Start.Format(time.RFC3339)
			if d > 0 && d <= lead && key != lastKey {
				title := fmt.Sprintf("Cours dans %d min", mins)
				msg := fmt.Sprintf("%s · Salle %s · %s", l.Summary, l.Location, l.Start.Format("15:04"))
				if err := publishNtfy(ntfyURL, topic, token, title, msg, []string{"bell"}, 3); err != nil {
					log.Printf("reminder ntfy error: %v", err)
				} else {
					lastKey = key
					log.Printf("reminder sent: %s at %s", l.Summary, l.Start.Format("15:04"))
				}
			}
		}
		time.Sleep(60 * time.Second)
	}
}
