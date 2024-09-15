package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/warthog618/go-gpiocdev"
	"github.com/warthog618/go-gpiocdev/device/rpi"
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
			"@unix("+config.Database.DbSock+")/"+config.Database.DbName+"?parseTime=true")
	if err != nil {
		log.Print("Error connecting to database: ", err)
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

	doorControl := createDoorControl(db, config)
	http.HandleFunc("/doorcontrol", page.wrap(doorControl.handle))

	http.ListenAndServe(":8080", nil)
}

func dbUpdater(db *sql.DB, statusUpdates StatusUpdateChan) {
	for {
		update := <-statusUpdates
		log.Print("Updating database:", update)
		_, err := db.Exec("INSERT INTO events (type) VALUES (?)",
			update.String())
		if err != nil {
			log.Print("Error writing to database: ", err)
		}
	}
}

func doorMonitor(config *Configuration) (chan *StatusRequest, StatusUpdateChan) {
	statusRequests := make(chan *StatusRequest)
	statusUpdates := make(StatusUpdateChan, 1)

	go func() {
		// Initialize the line and turn on the pull-up resistor.
		line, err := gpiocdev.RequestLine("gpiochip0", rpi.J8p16, gpiocdev.AsInput, gpiocdev.WithPullUp)
		if err != nil {
			log.Print("Error initializing line: ", err)
		}
		defer line.Close()

		doorStatus := STARTUP
		lastStatus := STARTUP
		statusUpdates <- doorStatus

		ticker := time.NewTicker(time.Duration(config.Polling.IntervalMillis) * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				var curStatus DoorStatus
				// read door status
				lineStatus, err := line.Value()
				if err != nil {
					log.Print("Error reading line: ", err)
				}
				if lineStatus == 0 {
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
