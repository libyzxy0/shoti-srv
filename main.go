package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"io"

	"github.com/joho/godotenv"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

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

// fetchAndInsertURLs fetches the URLs from an external API and inserts them into the database
func fetchAndInsertURLs(w http.ResponseWriter, r *http.Request) {
	// Fetch data from the external API
	resp, err := http.Get("https://shoti-server-production.up.railway.app/list")
	if err != nil {
		http.Error(w, "Error fetching URLs from external API", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the response body using io.ReadAll
	body, err := io.ReadAll(resp.Body) // Correct way to read data in Go 1.16 and later
	if err != nil {
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}

	// Parse the JSON response into a slice of URLs
	var urls []struct {
		URL string `json:"url"`
	}

	err = json.Unmarshal(body, &urls)
	if err != nil {
		http.Error(w, "Error parsing response JSON", http.StatusInternalServerError)
		return
	}

	// Insert URLs into the database
	for _, url := range urls {
		// Generate a new UUID for the URL
		urlID := uuid.New().String()

		// Insert the URL into the database
		query := "INSERT INTO urls (id, url) VALUES ($1, $2)"
		_, err := db.Exec(query, urlID, url.URL)
		if err != nil {
			http.Error(w, "Error inserting URL into database", http.StatusInternalServerError)
			return
		}
	}

	// Send a response confirming that URLs were fetched and inserted
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "URLs fetched and added to the database.")
}



func main() {
	initDB()

	http.HandleFunc("/new", addURL)
	http.HandleFunc("/get", getURLs)
	http.HandleFunc("/clr", clearURLs)
	http.HandleFunc("/fetch", fetchAndInsertURLs)
	
	fmt.Println("Server started on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
