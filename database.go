package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB // Capitalized to be visible (though not strictly necessary if in same package)

func InitDB(path string) error {
	_db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	DB = _db // Assign to global DB variable
	// Create a simple table for intercepted data
	query := `
	    CREATE TABLE IF NOT EXISTS requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		endpoint TEXT,
		payload TEXT,
		response TEXT
	    );`

	_, err = DB.Exec(query)
	return err
}

func saveRequest(requestEndPoint string, payload string, response string) {
	log.Println("Saving request to DB...")
	_, err := DB.Exec("INSERT INTO requests (endpoint, payload, response) VALUES (?, ?, ?)", requestEndPoint, payload, response)
	if err != nil {
		log.Println("Failed to save to DB: ", err)
	}
}

func getAllRequests() ([]map[string]interface{}, error) {
	rows, err := DB.Query("SELECT id, timestamp, endpoint, payload, response FROM requests")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var timestamp string
		var endpoint string
		var payload string
		var response string

		err := rows.Scan(&id, &timestamp, &endpoint, &payload, &response)
		if err != nil {
			return nil, err
		}

		record := map[string]interface{}{
			"id":        id,
			"timestamp": timestamp,
			"endpoint":  endpoint,
			"payload":   payload,
			"response":  response,
		}
		results = append(results, record)
	}
	return results, rows.Err()
}
