package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stianeikeland/go-rpio"
	"gopkg.in/natefinch/lumberjack.v2"
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
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(&lumberjack.Logger{
		Filename:   "/var/log/garagebot.log",
		MaxSize:    5, // megabytes
		MaxBackups: 10,
	})

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

	log.Print("Initializating notifier")
	go notifier(config, dispatcher.createListener())

	dispatcher.startDispatch()

	log.Print("Initializating http server")
	page := createPage(config)

	statusPage := createStatusPage(statusRequests, db)
	http.HandleFunc("/", page.wrap(statusPage.handle))

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
