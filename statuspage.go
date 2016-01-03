package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/abbot/go-http-auth"
	"github.com/kardianos/osext"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

type StatusPage struct {
	statusChan chan *StatusRequest
	db         *sql.DB
	template   *template.Template
}

type StatusPageData struct {
	Username   string
	DoorStatus string
	DoorLog    string
}

func createStatusPage(statusChan chan *StatusRequest, db *sql.DB) *StatusPage {
	executableFolder, err := osext.ExecutableFolder()
	if err != nil {
		log.Panic("Error finding executable folder:", err)
	}
	templatePath := filepath.Join(executableFolder, "../src/github.com/cghawthorne/garagebot/templates/statuspage.html")

	template, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Panic("Error loading template:", err)
	}

	statusPage := &StatusPage{statusChan: statusChan, db: db, template: template}
	return statusPage
}

func makeStatusRequest() *StatusRequest {
	return &StatusRequest{make(StatusUpdateChan, 1)}
}

func (s *StatusPage) handle(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// The "/" pattern matches everything, so we need to check
	// that we're at the root here.
	if r.Request.URL.Path != "/" {
		http.NotFound(w, &r.Request)
		return
	}

	// Only GET requests allowed
	if r.Request.Method != "GET" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
	}

	statusPageData := &StatusPageData{Username: r.Username}

	req := makeStatusRequest()
	s.statusChan <- req
	statusPageData.DoorStatus = (<-req.resultChan).String()

	var buffer bytes.Buffer
	rows, err := s.db.Query("SELECT ts, type, username FROM events ORDER BY ts DESC LIMIT 50")
	if err != nil {
		log.Print("Error querying database: ", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ts, eventType string
		var username sql.NullString
		if err := rows.Scan(&ts, &eventType, &username); err != nil {
			log.Print("Error scanning row: ", err)
			continue
		}
		fmt.Fprintln(&buffer, ts, " ", eventType, " ", username)
	}

	statusPageData.DoorLog = buffer.String()

	s.template.Execute(w, statusPageData)
}
