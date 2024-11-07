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
	ID        string `json:"id"`
	Region    string `json:"region"`
	Cover     string `json:"cover"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Author    struct {
		UniqueID  string `json:"unique_id"`
		Nickname  string `json:"nickname"`
		UserID    string `json:"id"`
	} `json:"author"`
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
    response, err := http.Get(fmt.Sprintf("https://tikwm.com/api?url=%s", url))
    if err != nil {
        return nil, err
    }
    defer response.Body.Close()

    var videoInfo VideoInfo
    err = json.NewDecoder(response.Body).Decode(&videoInfo)
    if err != nil {
        return nil, err
    }

    fmt.Println("Video Info:", videoInfo)

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

    fmt.Println(videoInfo)

    responseData := map[string]interface{}{
        "code":    200,
        "message": "success",
        "data": map[string]interface{}{
            "region":      videoInfo.Region,
            "url":         "https://www.tikwm.com/video/media/hdplay/" + videoInfo.ID + ".mp4",
            "cover":       videoInfo.Cover,
            "title":       videoInfo.Title,
            "duration":    fmt.Sprintf("%ds", videoInfo.Duration),
            "user": map[string]interface{}{
                "username": videoInfo.Author.UniqueID,
                "nickname": videoInfo.Author.Nickname,
                "userID":   videoInfo.Author.UserID,
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

func clearURLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method. Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := db.Exec("DELETE FROM urls")
	if err != nil {
		http.Error(w, "Error clearing URLs from database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "All URLs have been removed from the database.")
}

func main() {
	initDB()

	http.HandleFunc("/new", addURL)
	http.HandleFunc("/list", getURLs)
	http.HandleFunc("/clr", clearURLs)
	http.HandleFunc("/get", getVideoData)

	fmt.Println("Server started on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
