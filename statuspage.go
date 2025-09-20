package main

import (
	"database/sql"
	"github.com/kardianos/osext"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

type StatusPage struct {
	statusChan chan *StatusRequest
	db         *sql.DB
	template   *template.Template
}

type StatusPageData struct {
	Username   string
	DoorStatus string
	DoorLog    []*DoorEvent
}

type DoorEvent struct {
	Time     string
	Type     string
	Username sql.NullString
}

func createStatusPage(statusChan chan *StatusRequest, db *sql.DB) *StatusPage {
	executableFolder, err := osext.ExecutableFolder()
	if err != nil {
		log.Panic("Error finding executable folder:", err)
	}
	templatePath := filepath.Join(executableFolder, "templates/statuspage.html")

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

func (s *StatusPage) handle(w http.ResponseWriter, r *AuthenticatedRequest) {
	// The "/" pattern matches everything, so we need to check
	// that we're at the root here.
	if r.Request.URL.Path != "/" {
		http.NotFound(w, &r.Request)
		return
	}

	// Only GET requests allowed
	if r.Request.Method != "GET" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusPageData := &StatusPageData{Username: r.Username}

	req := makeStatusRequest()
	s.statusChan <- req
	statusPageData.DoorStatus = (<-req.resultChan).String()

	rows, err := s.db.Query("SELECT ts, type, username FROM events WHERE ts >= NOW() - INTERVAL 1 WEEK ORDER BY ts DESC LIMIT 1000")
	if err != nil {
		log.Print("Error querying database: ", err)
	}
	defer rows.Close()
	for rows.Next() {
		doorEvent := &DoorEvent{}
		statusPageData.DoorLog = append(statusPageData.DoorLog, doorEvent)
		var ts time.Time
		if err := rows.Scan(&ts, &doorEvent.Type, &doorEvent.Username); err != nil {
			log.Print("Error scanning row: ", err)
			continue
		}
		doorEvent.Time = ts.Format("2006-01-02 3:04 PM")
	}

	err = s.template.Execute(w, statusPageData)
	if err != nil {
		log.Print("Error executing template: ", err)
	}
}
