package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type VideoInfo struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    struct {
		ID               string `json:"id"`
		Region           string `json:"region"`
		Title            string `json:"title"`
		Cover            string `json:"cover"`
		AI_Dynamic_Cover string `json:"ai_dynamic_cover"`
		Origin_Cover     string `json:"origin_cover"`
		Duration         int    `json:"duration"`
		Play             string `json:"play"`
		WMPlay           string `json:"wmplay"`
		Size             int    `json:"size"`
		WMSize           int    `json:"wm_size"`
		Music            struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Play  string `json:"play"`
			Cover string `json:"cover"`
		} `json:"music_info"`
		PlayCount    int `json:"play_count"`
		DiggCount    int `json:"digg_count"`
		CommentCount int `json:"comment_count"`
		ShareCount   int `json:"share_count"`
		DownloadCount int `json:"download_count"`
		CollectCount int `json:"collect_count"`
		CreateTime   int64 `json:"create_time"`
		Author struct {
			ID       string `json:"id"`
			UniqueID string `json:"unique_id"`
			Nickname string `json:"nickname"`
			Avatar   string `json:"avatar"`
		} `json:"author"`
	} `json:"data"`
}

type VideoDataResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    struct {
		Region          string `json:"region"`
		URL             string `json:"url"`
		Cover           string `json:"cover"`
		Title           string `json:"title"`
		Duration        string `json:"duration"`
		User            struct {
			Username string `json:"username"`
			Nickname string `json:"nickname"`
			UserID   string `json:"userID"`
		} `json:"user"`
	} `json:"data"`
}

type URL struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

var db *sql.DB

func initDB() {
	if _, exists := os.LookupEnv("RAILWAY_ENVIRONMENT"); !exists {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file:", err)
		}
	}

	connStr := fmt.Sprintf(
		"user=%s password=%s host=%s dbname=%s sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Unable to connect to the database:", err)
	}

	fmt.Println("Connected to the database.")

	setupSchema()
}

func setupSchema() {
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id UUID PRIMARY KEY,
		url TEXT NOT NULL
	);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error setting up database schema:", err)
	}
	fmt.Println("Database schema set up successfully.")
}

func getRandomURL() (string, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&count)
	if err != nil {
		return "", fmt.Errorf("error getting URL count: %w", err)
	}

	if count == 0 {
		return "", fmt.Errorf("no URLs found in the database")
	}

	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(count) + 1

	var url string
	query := fmt.Sprintf("SELECT url FROM urls LIMIT 1 OFFSET %d", randomIndex-1)
	err = db.QueryRow(query).Scan(&url)
	if err != nil {
		return "", fmt.Errorf("error retrieving random URL: %w", err)
	}

	return url, nil
}

func getVideoInfo(url string) (*VideoInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://tikwm.com/api?url=%s", url), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching video info: %w", err)
	}
	defer response.Body.Close()

	var videoInfo VideoInfo
	err = json.NewDecoder(response.Body).Decode(&videoInfo)
	if err != nil {
		return nil, fmt.Errorf("error decoding video info: %w", err)
	}

	if videoInfo.Code != 0 {
		return nil, fmt.Errorf("API error: %s", videoInfo.Msg)
	}

	return &videoInfo, nil
}

func getVideoData(w http.ResponseWriter, r *http.Request) {
	randomURL, err := getRandomURL()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching random URL: %s", err), http.StatusInternalServerError)
		return
	}

	fmt.Println("Fetching video for URL:", randomURL)

	videoInfo, err := getVideoInfo(randomURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching video: %s", err), http.StatusInternalServerError)
		return
	}

	responseData := VideoDataResponse{
		Code:    200,
		Msg:     "success",
		Data: struct {
			Region    string `json:"region"`
			URL       string `json:"url"`
			Cover     string `json:"cover"`
			Title     string `json:"title"`
			Duration  string `json:"duration"`
			User      struct {
				Username string `json:"username"`
				Nickname string `json:"nickname"`
				UserID   string `json:"userID"`
			} `json:"user"`
		}{
			Region:   videoInfo.Data.Region,
			URL:      "https://www.tikwm.com/video/media/hdplay/" + videoInfo.Data.ID + ".mp4",
			Cover:    videoInfo.Data.Cover,
			Title:    videoInfo.Data.Title,
			Duration: fmt.Sprintf("%ds", videoInfo.Data.Duration),
			User: struct {
				Username string `json:"username"`
				Nickname string `json:"nickname"`
				UserID   string `json:"userID"`
			}{
				Username: videoInfo.Data.Author.UniqueID,
				Nickname: videoInfo.Data.Author.Nickname,
				UserID:   videoInfo.Data.Author.ID,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseData)
}

func addURL(w http.ResponseWriter, r *http.Request) {
	var url URL

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&url)
	if err != nil {
		if err.Error() == "EOF" {
			http.Error(w, "Empty request body", http.StatusBadRequest)
		} else {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
		}
		return
	}

	url.ID = uuid.New().String()

	query := "INSERT INTO urls (id, url) VALUES ($1, $2)"
	_, err = db.Exec(query, url.ID, url.URL)
	if err != nil {
		http.Error(w, "Error adding URL to database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(url)
}

func getURLs(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, url FROM urls")
	if err != nil {
		http.Error(w, "Error retrieving URLs from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var urls []URL
	for rows.Next() {
		var url URL
		if err := rows.Scan(&url.ID, &url.URL); err != nil {
			http.Error(w, "Error scanning URL from database", http.StatusInternalServerError)
			return
		}
		urls = append(urls, url)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}

func main() {
	initDB()

	http.HandleFunc("/api/new", addURL)
	http.HandleFunc("/api/list", getURLs)
	http.HandleFunc("/api/get", getVideoData)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
