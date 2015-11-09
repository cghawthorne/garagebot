package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stianeikeland/go-rpio"
	"log"
	"net/http"
	"time"
)

type DoorStatus int

const (
	OPEN DoorStatus = iota
	CLOSED
	STARTUP
)

func (ds DoorStatus) String() string {
	switch ds {
	case OPEN:
		return "open"
	case CLOSED:
		return "closed"
	case STARTUP:
		return "startup"
	default:
		return "unknown"
	}
}

const (
	pollInterval = 1 * time.Second // how frequently to poll door status
)

const (
	dbName     = "garagebot"
	dbUser     = "garagebot"
	dbPassword = "garagebot"
	dbSock     = "/var/run/mysqld/mysqld.sock"
)

type StatusRequest struct {
	resultChan chan DoorStatus
}

func main() {
	log.Print("Starting up")

	statusRequests := make(chan *StatusRequest)
	statusUpdates := make(chan DoorStatus, 1)
	go doorMonitor(statusRequests, statusUpdates)
	go dbUpdater(statusUpdates)

	http.Handle("/", &StatusPage{statusRequests})
	http.ListenAndServe(":80", nil)
}

func dbUpdater(statusUpdates chan DoorStatus) {
	db, err := sql.Open("mysql", dbUser+":"+dbPassword+"@unix("+dbSock+")/"+dbName)
	if err != nil {
		log.Print("Error connecting to database:", err)
	}

	for {
		update := <-statusUpdates
		log.Print("Updating database:", update)
		_, err = db.Exec("INSERT INTO events (type) VALUES (?)",
			update.String())
		if err != nil {
			log.Print("Error writing to database:", err)
		}
	}
}

func doorMonitor(statusRequests chan *StatusRequest, statusUpdates chan DoorStatus) {
	// Open and map memory to access gpio, check for errors.
	if err := rpio.Open(); err != nil {
		log.Panic(err)
	}
	// Unmap gpio memory when done.
	defer rpio.Close()

	// Initialize the pin and turn on the pull-up resistor.
	pin := rpio.Pin(23)
	pin.PullUp()

	doorStatus := STARTUP
	lastStatus := STARTUP
	statusUpdates <- doorStatus

	ticker := time.NewTicker(pollInterval)
	for {
		select {
		case <-ticker.C:
			var curStatus DoorStatus
			// read door status
			if pin.Read() == 0 {
				curStatus = CLOSED
			} else {
				curStatus = OPEN
			}

			if curStatus == lastStatus {
				// Same status twice in a row, assume it's real.
				if doorStatus != curStatus {
					doorStatus = curStatus
					statusUpdates <- doorStatus
				}
			}
			lastStatus = curStatus
		case req := <-statusRequests:
			req.resultChan <- doorStatus
		}
	}
}

type StatusPage struct {
	statusChan chan *StatusRequest
}

func makeStatusRequest() *StatusRequest {
	return &StatusRequest{make(chan DoorStatus, 1)}
}

func (s *StatusPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := makeStatusRequest()
	s.statusChan <- req
	doorStatus := <-req.resultChan

	fmt.Fprintln(w, "Garage door is:", doorStatus)
}
