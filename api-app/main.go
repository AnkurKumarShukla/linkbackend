package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

type Report struct {
	Timestamp       string `json:"timestamp"`
	StreamID        string `json:"stream_id"`
	ObservationsTS  int64  `json:"observations_ts"`
	BenchmarkPrice  string `json:"benchmark_price"`
	Bid             string `json:"bid"`
	Ask             string `json:"ask"`
	ValidFromTS     int64  `json:"valid_from_ts"`
	ExpiresAt       int64  `json:"expires_at"`
	LinkFee         string `json:"link_fee"`
	NativeFee       string `json:"native_fee"`
}

func main() {
	http.HandleFunc("/reports", func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("sqlite3", "./db/data.db")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer db.Close()

		rows, err := db.Query("SELECT * FROM reports ORDER BY timestamp DESC")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var reports []Report
		for rows.Next() {
			var rep Report
			rows.Scan(
				&rep.Timestamp, &rep.StreamID, &rep.ObservationsTS,
				&rep.BenchmarkPrice, &rep.Bid, &rep.Ask,
				&rep.ValidFromTS, &rep.ExpiresAt, &rep.LinkFee, &rep.NativeFee,
			)
			reports = append(reports, rep)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reports)
	})

	log.Println("API server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
