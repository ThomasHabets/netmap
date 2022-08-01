package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

func importLinks(ctx context.Context, js string, tx *sql.Tx) error {
	jqq := strings.ReplaceAll(`
.areaScopedLinkStateDb
| .[].lsa
| .[]
| select(.type=="Intra-Prefix")
| .advertisingRouter as $adv
| .prefix
| .[]
| [$adv,.prefix,.metric]
| @csv
`, "\n", " ")
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "jq", "-r", jqq)
	cmd.Stdin = bytes.NewBufferString(js)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	runErr := make(chan error, 1)
	go func() {
		defer w.Close()
		runErr <- cmd.Run()
	}()

	csvr := csv.NewReader(r)
	if _, err := tx.ExecContext(ctx, "DELETE FROM links"); err != nil {
		return err
	}
	for {
		record, err := csvr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO links(router,net,cost) VALUES(?,?,?)",
			record[0], record[1], record[2]); err != nil {
			return err
		}
	}
	return nil
}

func importNeigh(ctx context.Context, js string, tx *sql.Tx) error {
	jqq := strings.ReplaceAll(`.areaScopedLinkStateDb
| .[].lsa
| .[]
| select(.type=="Router")
| .advertisingRouter as $adv
| .lsaDescription | .[]
| [$adv,.interfaceId,.neighborRouterId,.neighborInterfaceId,.metric]
| @csv
`, "\n", " ")
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "jq", "-r", jqq)
	cmd.Stdin = bytes.NewBufferString(js)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	runErr := make(chan error, 1)
	go func() {
		defer w.Close()
		runErr <- cmd.Run()
	}()

	if _, err := tx.ExecContext(ctx, "DELETE FROM neigh"); err != nil {
		return err
	}
	csvr := csv.NewReader(r)
	for {
		record, err := csvr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO neigh(node1_id, link1, node2_id, link2)
VALUES(?,?,?,?)
`, record[0], record[1], record[2], record[3]); err != nil {
			return err
		}
	}
	return <-runErr
}

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

	log.Infof("Running…")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	js := func() string {
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		return string(b)
	}()
	if err := importLinks(ctx, js, tx); err != nil {
		log.Fatal(err)
	}
	if err := importNeigh(ctx, js, tx); err != nil {
		log.Fatal(err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
}
