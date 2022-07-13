package main

import (
	"net/http"
	"sort"
	"encoding/json"
	"fmt"
	"context"
	"io/ioutil"
	"strings"
	"bytes"
	"embed"
	"flag"
	"database/sql"
	"html/template"
	texttemplate "text/template"
	"os/exec"

	log "github.com/sirupsen/logrus"
	_ "github.com/mattn/go-sqlite3"
	"github.com/gorilla/mux"
)

var (
	//go:embed root.html
	rootTmplString string

	//go:embed graph.dot
	graphTmplString string

	rootTmpl = template.Must(template.New("root").Parse(rootTmplString))
	graphTmpl = texttemplate.Must(texttemplate.New("graph").Parse(graphTmplString))

	//go:embed static/netmap.css
	//go:embed static/netmap.js
	fs embed.FS
	
	db *sql.DB
)

func rootHandler(w http.ResponseWriter, req *http.Request) {
	graph, err := generateGraphData(req.Context())
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := rootTmpl.Execute(w, graph); err != nil {
		log.Error(err)
		return
	}
}

type Router struct {
	ID string
	Name string
	Pos string
}

type Net struct {
	ID string
	Pos string
}

type Link struct {
	Router string
	Net string
}

type graphData struct {
	Router []Router
	Net []Net
	Link []Link
}


func getPositions(ctx context.Context) (map[string]string, error) {
	ret := make(map[string]string)
	rows, err := db.QueryContext(ctx, `SELECT id,x,y FROM pos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var x,y int
		if err := rows.Scan(&k, &x, &y); err != nil {
			return nil, err
		}
		ret[k] = fmt.Sprintf("%d,%d", x,y)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret,nil
}

func getNames(ctx context.Context) (map[string]string, error) {
	ret := make(map[string]string)
	rows, err := db.QueryContext(ctx, `SELECT node_id,name FROM nodenames ORDER BY node_id`)
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
	return ret,nil
}

func generateGraphData(ctx context.Context) (*graphData, error) {
	poss, err := getPositions(ctx)
	if err != nil {
		return nil, err
	}

	names, err := getNames(ctx)
	if err != nil {
		return nil, err
	}
	
	var graph graphData
	rows, err := db.QueryContext(ctx, "SELECT router, net, cost FROM links")
	if err != nil {
		return nil,err
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
			e:=Router{
				ID: router,
				Name: names[router],
				Pos: poss[router],
			}
			graph.Router =append(graph.Router, e)
		}
		if !seen[link] {
			e:=Net{
				ID: link,
				Pos: poss[link],
			}
			graph.Net = append(graph.Net, e)
		}
		graph.Link = append(graph.Link, Link{
			Router: router,
			Net: link,
		})
		seen[router] = true
		seen[link] = true
	}
	sort.Slice(graph.Router, func(i, j int) bool {
		return graph.Router[i].Name < graph.Router[j].Name
	})
	sort.Slice(graph.Net, func(i, j int) bool {
		return graph.Net[i].ID < graph.Net[j].ID
	})
	return &graph, nil
}
	

func generateDot(ctx context.Context) (string, error) {
	graph, err:=generateGraphData(ctx)
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var u update
	if err := json.Unmarshal(b, &u);err!=nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := db.ExecContext(req.Context(), `UPDATE pos SET x=?,y=? WHERE id=?`,u.X,u.Y,id);err!= nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func renderHandler(w http.ResponseWriter, req *http.Request) {
	dot, err := generateDot(req.Context())
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
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
	cmd:=exec.CommandContext(req.Context(), "dot", "-T" + format)
	cmd.Stdout = w
	cmd.Stdin = bytes.NewBufferString(dot)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil{
		log.Errorf("graphviz: %v: %s\n%s", err, stderr.String(), dot)
		return
	}
}

func main() {
	flag.Parse()

	var err error
	db, err = sql.Open("sqlite3", "netmap.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
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
