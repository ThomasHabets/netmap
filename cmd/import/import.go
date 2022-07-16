package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"io"
	"os"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	db, err := sql.Open("sqlite3", "netmap.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		log.Fatalf("Failed to turn on foreign keys: %v", err)
	}

	r := csv.NewReader(os.Stdin)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM links"); err != nil {
		log.Fatal(err)
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO links(router,net,cost) VALUES(?,?,?)",
			record[0], record[1], record[2]); err != nil {
			log.Fatal(err)
		}
		log.Info(record)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
}
