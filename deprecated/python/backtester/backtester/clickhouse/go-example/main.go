package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Candle struct {
	Timestamp time.Time `ch:"timestamp"`
	High      float64   `ch:"high"`
	Low       float64   `ch:"low"`
	Open      float64   `ch:"open"`
	Close     float64   `ch:"close"`
}

func GenerateCandle() *Candle {
	return &Candle{
		Timestamp: time.Now(),
		High:      rand.Float64() * 100,
		Low:       rand.Float64() * 100,
		Open:      rand.Float64() * 100,
		Close:     rand.Float64() * 100,
	}
}

func QueryCandles(ctx context.Context, conn driver.Conn) {
	// Query the candles
	rows, err := conn.Query(ctx, "SELECT timestamp, high, low, open, close FROM candles ORDER BY timestamp DESC LIMIT 10")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Iterate through the rows
	var candles []Candle

	for rows.Next() {
		var candle Candle
		if err := rows.ScanStruct(&candle); err != nil {
			log.Fatal(err)
		}

		candles = append(candles, candle)
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Retrieved %d candles\n", len(candles))

	for _, candle := range candles {
		fmt.Printf("Timestamp: %v, High: %v, Low: %v, Open: %v, Close: %v\n", candle.Timestamp, candle.High, candle.Low, candle.Open, candle.Close)
	}
}

func InsertCandles(conn driver.Conn, candles []*Candle) error {
	ctx := context.Background()
	batch, err := conn.PrepareBatch(ctx, "INSERT INTO candles (timestamp, high, low, open, close)")
	if err != nil {
		return err
	}

	for _, candle := range candles {
		err := batch.Append(candle.Timestamp, candle.High, candle.Low, candle.Open, candle.Close)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}

func createSchemaIfNotExists(conn driver.Conn) {
	// Define the schema (create a table)
	createTableQuery := `
        CREATE TABLE IF NOT EXISTS candles (
			timestamp DateTime,
			high Float64,
			low Float64,
			open Float64,
			close Float64
		) ENGINE = MergeTree()
		ORDER BY timestamp;
    `

	// Execute the query
	if err := conn.Exec(context.Background(), createTableQuery); err != nil {
		log.Fatal(err)
	}

	log.Println("Table created successfully")
}

func insertCandles(conn driver.Conn) {

	// Example usage of GenerateCandle and InsertCandles
	candles := []*Candle{
		GenerateCandle(),
		GenerateCandle(),
	}

	err := InsertCandles(conn, candles)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Candles inserted successfully")
}

func main() {
	ctx := context.Background()

	// Create a connection to the ClickHouse server
	conn, err := connect()
	if err != nil {
		panic(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	createSchemaIfNotExists(conn)
	// insertCandles()
	QueryCandles(ctx, conn)
}

func connect() (driver.Conn, error) {
	var (
		ctx       = context.Background()
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{"localhost:9000"},
			Auth: clickhouse.Auth{
				Database: "default",
				Username: "default",
				Password: "",
			},
			ClientInfo: clickhouse.ClientInfo{
				Products: []struct {
					Name    string
					Version string
				}{
					{Name: "slack-trading-go-client", Version: "0.1"},
				},
			},
			Debugf: func(format string, v ...interface{}) {
				fmt.Printf(format, v)
			},
		})
	)

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return nil, err
	}

	return conn, nil
}
