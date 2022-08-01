package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

const (
	// TODO
	mapName = "main"

	defaultLayout = "neato"
)

var (
	//go:embed root.html
	rootTmplString string

	//go:embed graph.dot
	graphTmplString string

	rootTmpl  = template.Must(template.New("root").Parse(rootTmplString))
	graphTmpl = texttemplate.Must(texttemplate.New("graph").Parse(graphTmplString))

	//go:embed static/netmap.css
	//go:embed static/netmap.js
	fs embed.FS

	db *sql.DB

	dbName = flag.String("db", "netmap.sqlite", "SQLite database.")
)

func rootHandler(w http.ResponseWriter, req *http.Request) {
	graph, err := generateGraphData(req.Context(), defaultLayout)
	if err != nil {
		log.Error(err)
		http.Error(w, "Failed to generate graph", http.StatusInternalServerError)
		return
	}
	if err := rootTmpl.Execute(w, graph); err != nil {
		log.Error(err)
		return
	}
}

type Router struct {
	ID   string
	Name string
	Pos  string
}

type Net struct {
	ID      string
	Pos     string
	Missing bool
}

type Link struct {
	Router string
	Net    string
	Cost   int
}

type graphData struct {
	Layout string
	Router []Router
	Net    []Net
	Link   []Link
	Neigh  []neigh
}

func getPositions(ctx context.Context) (map[string]string, error) {
	ret := make(map[string]string)
	rows, err := db.QueryContext(ctx, `
SELECT node_id,x,y
FROM pos
NATURAL JOIN maps
WHERE maps.name=?
ORDER BY node_id
`, mapName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var x, y int
		if err := rows.Scan(&k, &x, &y); err != nil {
			return nil, err
		}
		ret[k] = fmt.Sprintf("%d,%d", x, y)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func getNames(ctx context.Context) (map[string]string, error) {
	ret := make(map[string]string)
	rows, err := db.QueryContext(ctx, `
SELECT node_id,name
FROM nodenames
ORDER BY node_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var name string
		if err := rows.Scan(&k, &name); err != nil {
			return nil, err
		}
		ret[k] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

type neigh struct {
	Node1 string
	Link1 string
	Node2 string
	Link2 string
}

func getNeigh(ctx context.Context) ([]neigh, error) {
	var ret []neigh
	rows, err := db.QueryContext(ctx, `
SELECT node1_id,link1, node2_id, link2
FROM neigh
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p neigh
		if err := rows.Scan(&p.Node1, &p.Link1, &p.Node2, &p.Link2); err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func generateGraphData(ctx context.Context, layout string) (*graphData, error) {
	poss, err := getPositions(ctx)
	if err != nil {
		return nil, err
	}

	names, err := getNames(ctx)
	if err != nil {
		return nil, err
	}

	neigh, err := getNeigh(ctx)
	if err != nil {
		return nil, err
	}

	graph := graphData{
		Layout: layout,
		Neigh:  neigh,
	}
	rows, err := db.QueryContext(ctx, `
SELECT links.router, net, cost FROM links
JOIN mapnodes ON links.router=mapnodes.router
JOIN maps ON mapnodes.map_id=mapnodes.map_id
WHERE maps.name=?
AND links.net IN (
  SELECT mapnodes.router
  FROM mapnodes
  JOIN maps ON mapnodes.map_id=mapnodes.map_id
  WHERE maps.name=?
)
`, mapName, mapName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[string]bool)
	for rows.Next() {
		var router, link string
		var cost int
		if err := rows.Scan(&router, &link, &cost); err != nil {
			return nil, err
		}
		if !seen[router] {
			e := Router{
				ID:   router,
				Name: names[router],
				Pos:  poss[router],
			}
			graph.Router = append(graph.Router, e)
		}
		if !seen[link] {
			e := Net{
				ID:  link,
				Pos: poss[link],
			}
			graph.Net = append(graph.Net, e)
		}
		graph.Link = append(graph.Link, Link{
			Router: router,
			Net:    link,
			Cost:   cost,
		})
		seen[router] = true
		seen[link] = true
	}
	for e, pos := range poss {
		if !seen[e] {
			graph.Net = append(graph.Net, Net{
				ID:      e,
				Pos:     pos,
				Missing: true,
			})
		}
	}
	sort.Slice(graph.Router, func(i, j int) bool {
		return graph.Router[i].Name < graph.Router[j].Name
	})
	sort.Slice(graph.Net, func(i, j int) bool {
		return graph.Net[i].ID < graph.Net[j].ID
	})
	return &graph, nil
}

func generateDot(ctx context.Context, layout string) (string, error) {
	graph, err := generateGraphData(ctx, layout)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := graphTmpl.Execute(&buf, graph); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type update struct {
	X string `json:"x"`
	Y string `json:"y"`
}

func updateHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := strings.ReplaceAll(vars["id"], "__SLASH__", "/")
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "Failed to read request", http.StatusInternalServerError)
		return
	}
	var u update
	if err := json.Unmarshal(b, &u); err != nil {
		log.Error(err)
		http.Error(w, "Failed to parse request JSON", http.StatusBadRequest)
		return
	}
	if res, err := db.ExecContext(req.Context(), `
UPDATE pos SET x=?,y=?
WHERE node_id=?
AND map_id=(SELECT map_id FROM maps WHERE name=?)`, u.X, u.Y, id, mapName); err != nil {
		log.Error(err)
		http.Error(w, "Failed to update", http.StatusInternalServerError)
		return
	} else if n, err := res.RowsAffected(); err != nil {
		log.Warningf("Failed to get rows affected for %q", id)
		// Pretend to caller that it succeeded.
	} else if n == 0 {
		if _, err := db.ExecContext(req.Context(), `
INSERT INTO pos(node_id,x,y,map_id)
VALUES(?,?,?, (SELECT map_id FROM maps WHERE name=?))
`, id, u.X, u.Y, mapName); err != nil {
			log.Error(err)
			http.Error(w, "Failed to insert", http.StatusInternalServerError)
			return
		}
	} else if n > 1 {
		log.Errorf("More than one row affected for %q? WUT?!", id)
		http.Error(w, "Multi update, wat?", http.StatusInternalServerError)
		return
	}
}

func renderHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	layout := defaultLayout
	if v := req.Form.Get("layout"); map[string]bool{
		"neato": true,
		"circo": true,
		"fdp":   true,
	}[v] {
		layout = v
	}
	dot, err := generateDot(req.Context(), layout)
	if err != nil {
		log.Error(err)
		http.Error(w, "Failed to generate dot", http.StatusInternalServerError)
		return
	}

	format := "png"
	w.Header().Set("Content-Type", "image/png")
	for _, t := range strings.Split(req.Header.Get("Accept"), ",") {
		wparms := strings.Split(t, ";")
		typ := wparms[0]
		// TODO: what is the header to check?
		// application/xhtml+xml ?
		// text/html?
		if typ == "image/svg+xml" || typ == "application/xml" {
			w.Header().Set("Content-Type", "image/svg+xml")
			format = "svg"
			break
		}
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(req.Context(), "dot", "-T"+format)
	cmd.Stdout = w
	cmd.Stdin = io.TeeReader(bytes.NewBufferString(dot), os.Stdout)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("graphviz: %v: %s\n%s", err, stderr.String(), dot)
		return
	}
}

func main() {
	flag.Parse()
	ctx := context.Background()

	var err error
	db, err = sql.Open("sqlite3", *dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for _, line := range []string{
		`PRAGMA foreign_keys = ON`,
		// `.load /usr/lib/sqlite3/pcre.so`,
	} {
		if _, err := db.ExecContext(ctx, line); err != nil {
			log.Fatalf("Failed to turn on foreign keys: %v", err)
		}
	}

	log.Info("Running…")

	r := mux.NewRouter()
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/render", renderHandler)
	r.HandleFunc("/update/{id}", updateHandler)
	r.PathPrefix("/static/").Handler(http.FileServer(http.FS(fs)))
	//r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(dir))))
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
