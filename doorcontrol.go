package main

import (
	"database/sql"
	"github.com/abbot/go-http-auth"
	"github.com/stianeikeland/go-rpio"
	"log"
	"net/http"
	"time"
)

type DoorControl struct {
	doorControlRequest chan *DoorControlRequest
}

type DoorControlState int

const (
	IDLE DoorControlState = iota
	CIRCUIT_ACTIVE
)

func (state DoorControlState) String() string {
	switch state {
	case IDLE:
		return "idle"
	case CIRCUIT_ACTIVE:
		return "circuit_active"
	default:
		return "unknown"
	}
}

type DoorControlRequest struct {
	Username string
	State    DoorControlState
}

func createDoorControl(db *sql.DB, config *Configuration) *DoorControl {
	doorControl := &DoorControl{make(chan *DoorControlRequest)}
	pin := rpio.Pin(24)
	pin.Output()
	pin.High() // High == relay off

	go func() {
		state := IDLE
		for req := range doorControl.doorControlRequest {
			log.Printf("Got door control request: %v", req)
			switch req.State {
			case IDLE:
				if state != CIRCUIT_ACTIVE {
					log.Printf("Ignoring invalid state transition (%v to %v)", state, req.State)
					break
				}
				log.Print("Deactivating circuit")
				_, err := db.Exec("INSERT INTO events (type, username) VALUES (?, ?)",
					"deactivate", req.Username)
				if err != nil {
					log.Print("Error writing to database:", err)
				}
				// turn off circuit
				pin.High()
				state = IDLE
			case CIRCUIT_ACTIVE:
				if state != IDLE {
					log.Printf("Ignoring invalid state transition (%v to %v)", state, req.State)
					break
				}
				log.Print("Activating circuit")
				_, err := db.Exec("INSERT INTO events (type, username) VALUES (?, ?)",
					"activate", req.Username)
				if err != nil {
					log.Print("Error writing to database:", err)
				}
				// turn on circuit
				pin.Low()
				state = CIRCUIT_ACTIVE
				// turn off the circuit after a delay
				time.AfterFunc(time.Duration(config.DoorControl.ActivationPeriodMillis)*time.Millisecond, func() {
					log.Print("Requesting circuit deactivation")
					doorControl.doorControlRequest <- &DoorControlRequest{req.Username, IDLE}
				})
			default:
				log.Printf("Ignoring request for unknown state %v", req.State)
			}
		}
	}()

	return doorControl
}

func (d *DoorControl) handle(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	// Only POST requests allowed
	if r.Request.Method != "POST" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Print("Requesting circuit activation")
	d.doorControlRequest <- &DoorControlRequest{r.Username, CIRCUIT_ACTIVE}
	http.Redirect(w, &r.Request, "", http.StatusFound)
}
