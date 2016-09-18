package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func kraken(data interface{}, path ...string) error {
	url := strings.Join(append([]string{"https://api.twitch.tv/kraken"}, path...), "/")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Client-ID", CLIENT_ID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(data)
}

func roundToSeconds(d time.Duration) time.Duration {
	return ((d + time.Second/2) / time.Second) * time.Second
}

func getUptime(channel string) string {
	var data struct {
		Stream struct {
			CreatedAt time.Time `json:"created_at"`
		}
	}
	err := kraken(&data, "streams", channel)
	if err != nil {
		log.Printf("getUptime=%v", err)
		return fmt.Sprintf("%s is not online", channel)
	}

	// if t, err := time.Parse(time.RFC3339, u); err == nil {}
	return roundToSeconds(time.Since(data.Stream.CreatedAt)).String()
}

func getGame(channel string, rating bool) string {
	var data struct {
		Game string
	}
	err := kraken(&data, "channels", channel)
	if err != nil {
		log.Printf("getGame=%v", err)
		return "API is down"
	}

	if rating {
		return getRating(data.Game)
	}
	return data.Game
}

var ratings = struct {
	sync.Mutex
	m map[string]string
}{m: make(map[string]string)}

func getRating(game string) string {
	ratings.Lock()
	defer ratings.Unlock()

	if r, ok := ratings.m[game]; ok {
		return r
	}

	q := url.Values{
		"count": {"1"},
		"game":  {game},
	}
	if req, err := http.NewRequest("GET", "https://videogamesrating.p.mashape.com/get.php?"+q.Encode(), nil); err == nil {
		req.Header.Add("X-Mashape-Key", MASHAPE_KEY)
		req.Header.Add("Accept", "application/json")
		if resp, err := http.DefaultClient.Do(req); err == nil {
			defer resp.Body.Close()
			var data []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data) > 0 {
				if score, ok := data[0]["score"].(string); ok && score != "" {
					r := fmt.Sprintf("%s [Rating: %s]", game, score)
					ratings.m[game] = r
					return r
				}
			} else {
				log.Print(err)
			}
		}
	}

	return game
}
