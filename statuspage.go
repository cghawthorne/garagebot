package main

import (
	"database/sql"
	"encoding/json"
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

type StatusPageTemplateData struct {
	Username string
}

type StatusResponse struct {
	DoorStatus string            `json:"doorStatus"`
	DoorLog    []DoorEventRecord `json:"doorLog"`
}

type DoorEventRecord struct {
	Time     string  `json:"time"`
	Type     string  `json:"type"`
	Username *string `json:"username,omitempty"`
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

	templateData := &StatusPageTemplateData{Username: r.Username}

	err := s.template.Execute(w, templateData)
	if err != nil {
		log.Print("Error executing template: ", err)
	}
}

func (s *StatusPage) loadStatusData() (*StatusResponse, error) {
	req := makeStatusRequest()
	s.statusChan <- req
	doorStatus := (<-req.resultChan).String()

	rows, err := s.db.Query("SELECT ts, type, username FROM events WHERE ts >= NOW() - INTERVAL 1 WEEK ORDER BY ts DESC LIMIT 1000")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	doorLog := make([]DoorEventRecord, 0)
	for rows.Next() {
		var (
			ts       time.Time
			event    string
			username sql.NullString
		)
		if err := rows.Scan(&ts, &event, &username); err != nil {
			log.Print("Error scanning row: ", err)
			continue
		}
		record := DoorEventRecord{
			Time: ts.Format("2006-01-02 3:04 PM"),
			Type: event,
		}
		if username.Valid {
			record.Username = &username.String
		}
		doorLog = append(doorLog, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	statusResponse := &StatusResponse{
		DoorStatus: doorStatus,
		DoorLog:    doorLog,
	}
	return statusResponse, nil
}

func (s *StatusPage) apiStatus(w http.ResponseWriter, r *AuthenticatedRequest) {
	if r.Request.Method != "GET" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusData, err := s.loadStatusData()
	if err != nil {
		log.Print("Error loading status data: ", err)
		http.Error(w, "Unable to load recent events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(statusData); err != nil {
		log.Print("Error encoding status data: ", err)
	}
}
