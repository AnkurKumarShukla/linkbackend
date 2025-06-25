package main

import (
    "context"
    "database/sql"
    "encoding/csv"
    "fmt"
    "log"
    "os"
    "strconv"
    "time"

    _ "github.com/mattn/go-sqlite3"
    streams "github.com/smartcontractkit/data-streams-sdk/go"
    feed "github.com/smartcontractkit/data-streams-sdk/go/feed"
    report "github.com/smartcontractkit/data-streams-sdk/go/report"
    v3 "github.com/smartcontractkit/data-streams-sdk/go/report/v3"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: go run main.go [StreamID1] [StreamID2] ...")
        os.Exit(1)
    }

    apiKey := "a625ef1a-41e0-41e4-a452-db5494203e3f"
	apiSecret := "z94)?-Vi9cjUAZHF9Oq0-l=6f7U5RMav2q4C3SEY=wC(2)@?wKSPj)_63Xw0vVvzYz58x34@f-9gZFDmbCnhoeRcey^5+rVr0qWu4>MqN-Na5-Sp-=M)S-0n@845zDKz"
    if apiKey == "" || apiSecret == "" {
        log.Fatal("API_KEY and API_SECRET must be set as environment variables.")
    }

    cfg := streams.Config{
        ApiKey:    apiKey,
        ApiSecret: apiSecret,
        WsURL:     "wss://ws.testnet-dataengine.chain.link",
        Logger:    streams.LogPrintf,
    }

    client, err := streams.New(cfg)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    var ids []feed.ID
    for _, arg := range os.Args[1:] {
        var fid feed.ID
        if err := fid.FromString(arg); err != nil {
            log.Fatalf("Invalid stream ID %s: %v", arg, err)
        }
        ids = append(ids, fid)
    }

    ctx:= context.Background()

    stream, err := client.Stream(ctx, ids)
    if err != nil {
        log.Fatalf("Failed to subscribe: %v", err)
    }
    defer stream.Close()

    db, err := sql.Open("sqlite3", "./db/stream_data.db")
    if err != nil {
        log.Fatalf("Failed to open SQLite DB: %v", err)
    }
    defer db.Close()

    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS reports (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT,
            stream_id TEXT,
            observations_timestamp INTEGER,
            benchmark_price TEXT,
            bid TEXT,
            ask TEXT,
            valid_from_timestamp INTEGER,
            expires_at INTEGER,
            link_fee TEXT,
            native_fee TEXT,
            synthetic_oi REAL
        )
    `)
    if err != nil {
        log.Fatalf("Failed to create table: %v", err)
    }

    csvFile, err := os.Create("decoded_reports.csv")
    if err != nil {
        log.Fatalf("Failed to create CSV file: %v", err)
    }
    defer csvFile.Close()

    writer := csv.NewWriter(csvFile)
    defer writer.Flush()
    writer.Write([]string{
        "Timestamp", "StreamID", "ObservationsTimestamp", "BenchmarkPrice",
        "Bid", "Ask", "ValidFromTimestamp", "ExpiresAt", "LinkFee", "NativeFee", "SyntheticOI",
    })

    var lastMid float64 = 0.0
    var syntheticOI float64 = 0.0
    const beta1 = 0.5
    const beta2 = 0.5

    for {
        reportResponse, err := stream.Read(context.Background())
        if err != nil {
            log.Printf("Error reading from stream: %v", err)
            continue
        }

        decodedReport, decodeErr := report.Decode[v3.Data](reportResponse.FullReport)
        if decodeErr != nil {
            log.Printf("Failed to decode report: %v", decodeErr)
            continue
        }

        bid, _ := strconv.ParseFloat(decodedReport.Data.Bid.String(), 64)
        ask, _ := strconv.ParseFloat(decodedReport.Data.Ask.String(), 64)
        mid := (bid + ask) / 2.0
        spread := ask - bid
        deltaMid := mid - lastMid
        deltaOI := beta1*(-spread) + beta2*deltaMid
        syntheticOI += deltaOI
        lastMid = mid

        row := []string{
            time.Now().Format(time.RFC3339),
            reportResponse.FeedID.String(),
            strconv.FormatInt(int64(decodedReport.Data.ObservationsTimestamp), 10),
            decodedReport.Data.BenchmarkPrice.String(),
            decodedReport.Data.Bid.String(),
            decodedReport.Data.Ask.String(),
            strconv.FormatInt(int64(decodedReport.Data.ValidFromTimestamp), 10),
            strconv.FormatInt(int64(decodedReport.Data.ExpiresAt), 10),
            decodedReport.Data.LinkFee.String(),
            decodedReport.Data.NativeFee.String(),
            fmt.Sprintf("%f", syntheticOI),
        }
        writer.Write(row)
        writer.Flush()

        stmt, err := db.Prepare(`
            INSERT INTO reports (
                timestamp, stream_id, observations_timestamp, benchmark_price, bid, ask,
                valid_from_timestamp, expires_at, link_fee, native_fee, synthetic_oi
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `)
        if err != nil {
            log.Fatal(err)
        }

        _, err = stmt.Exec(
            time.Now().Format(time.RFC3339),
            reportResponse.FeedID.String(),
            decodedReport.Data.ObservationsTimestamp,
            decodedReport.Data.BenchmarkPrice.String(),
            decodedReport.Data.Bid.String(),
            decodedReport.Data.Ask.String(),
            decodedReport.Data.ValidFromTimestamp,
            decodedReport.Data.ExpiresAt,
            decodedReport.Data.LinkFee.String(),
            decodedReport.Data.NativeFee.String(),
            syntheticOI,
        )
        if err != nil {
            log.Printf("DB insert error: %v", err)
        }

        cfg.Logger("Report written to CSV and DB: %+v\n", row)
    }
}
