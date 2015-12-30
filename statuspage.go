package main

import (
	"database/sql"
	"fmt"
	"github.com/abbot/go-http-auth"
	"log"
	"net/http"
)

type StatusPage struct {
	statusChan chan *StatusRequest
	db         *sql.DB
}

func createStatusPage(statusChan chan *StatusRequest, db *sql.DB) *StatusPage {
	statusPage := &StatusPage{statusChan: statusChan, db: db}
	return statusPage
}

func makeStatusRequest() *StatusRequest {
	return &StatusRequest{make(StatusUpdateChan, 1)}
}

func (s *StatusPage) handle(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	req := makeStatusRequest()
	s.statusChan <- req
	doorStatus := <-req.resultChan
	fmt.Fprintln(w, "Hello,", r.Username)
	fmt.Fprintln(w, "Garage door is:", doorStatus)
	fmt.Fprintln(w)

	rows, err := s.db.Query("SELECT ts, type FROM events ORDER BY ts DESC LIMIT 50")
	if err != nil {
		log.Print("Error querying database: ", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ts, eventType string
		if err := rows.Scan(&ts, &eventType); err != nil {
			log.Print("Error scanning row: ", err)
			continue
		}
		fmt.Fprintln(w, ts, " ", eventType)
	}
}
