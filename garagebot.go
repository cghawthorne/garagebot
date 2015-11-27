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

type StatusRequest struct {
	resultChan StatusUpdateChan
}

type StatusUpdateChan chan DoorStatus

func main() {
	log.Print("Starting up")

	config := readConfiguration()

	db, err := sql.Open("mysql",
		config.Database.DbUser+":"+config.Database.DbPassword+
			"@unix("+config.Database.DbSock+")/"+config.Database.DbName)
	if err != nil {
		log.Print("Error connecting to database:", err)
	}

	log.Print("Initializing door monitor")
	statusRequests, statusUpdates := doorMonitor(config)

	log.Print("Initializating dispatcher")
	dispatcher := createDispatcher(statusUpdates)

	log.Print("Initializating database updater")
	go dbUpdater(db, dispatcher.createListener())

	log.Print("Initializating http server")
	http.Handle("/", &StatusPage{statusRequests, db})
	http.ListenAndServe(":80", nil)
}

func dbUpdater(db *sql.DB, statusUpdates StatusUpdateChan) {
	for {
		update := <-statusUpdates
		log.Print("Updating database:", update)
		_, err := db.Exec("INSERT INTO events (type) VALUES (?)",
			update.String())
		if err != nil {
			log.Print("Error writing to database:", err)
		}
	}
}

func doorMonitor(config *Configuration) (chan *StatusRequest, StatusUpdateChan) {
	statusRequests := make(chan *StatusRequest)
	statusUpdates := make(StatusUpdateChan, 1)

	go func() {
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

		ticker := time.NewTicker(time.Duration(config.Polling.IntervalMillis) * time.Millisecond)
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
	}()

	return statusRequests, statusUpdates
}

type StatusPage struct {
	statusChan chan *StatusRequest
	db         *sql.DB
}

func makeStatusRequest() *StatusRequest {
	return &StatusRequest{make(StatusUpdateChan, 1)}
}

func (s *StatusPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := makeStatusRequest()
	s.statusChan <- req
	doorStatus := <-req.resultChan
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
