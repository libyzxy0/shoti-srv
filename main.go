package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type URL struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

var db *sql.DB

// initDB initializes the database connection and ensures the schema is set up
func initDB() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Build the database connection string
	connStr := fmt.Sprintf(
		"user=%s password=%s host=%s dbname=%s sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	// Initialize database connection
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Unable to connect to the database:", err)
	}

	fmt.Println("Connected to the database.")

	// Set up the database schema
	setupSchema()
}

// setupSchema sets up the necessary schema in the database
func setupSchema() {
	// Define the schema creation query
	query := `
	CREATE TABLE IF NOT EXISTS urls (
		id UUID PRIMARY KEY,
		url TEXT NOT NULL
	);
	`

	// Execute the schema creation query
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error setting up database schema:", err)
	}
	fmt.Println("Database schema set up successfully.")
}

// addURL handles adding a new URL to the database
func addURL(w http.ResponseWriter, r *http.Request) {
	var url URL

	// Ensure the content type is application/json
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	// Parse the incoming JSON request body into the URL struct
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

	// Generate a new UUID for the URL
	url.ID = uuid.New().String()

	// Insert the URL into the database
	query := "INSERT INTO urls (id, url) VALUES ($1, $2)"
	_, err = db.Exec(query, url.ID, url.URL)
	if err != nil {
		http.Error(w, "Error adding URL to database", http.StatusInternalServerError)
		return
	}

	// Respond with the added URL
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(url)
}

// getURLs retrieves all URLs from the database
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

	http.HandleFunc("/new", addURL)
	http.HandleFunc("/get", getURLs)

	fmt.Println("Starting server on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
