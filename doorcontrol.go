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
	db                 *sql.DB
	doorControlRequest chan DoorControlState
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

func createDoorControl(db *sql.DB, config *Configuration) *DoorControl {
	doorControl := &DoorControl{db, make(chan DoorControlState)}
	pin := rpio.Pin(24)
	pin.Output()
	pin.High() // High == relay off

	go func() {
		state := IDLE
		for req := range doorControl.doorControlRequest {
			log.Printf("Got door control request: %s", req)
			switch req {
			case IDLE:
				if state != CIRCUIT_ACTIVE {
					log.Printf("Ignoring invalid state transition (%s to %s)", state, req)
					break
				}
				// turn off circuit
				log.Print("Deactivating circuit")
				pin.High()
				state = IDLE
			case CIRCUIT_ACTIVE:
				if state != IDLE {
					log.Printf("Ignoring invalid state transition (%s to %s)", state, req)
					break
				}
				log.Print("Activating circuit")
				// turn on circuit
				pin.Low()
				state = CIRCUIT_ACTIVE
				// turn off the circuit after a delay
				time.AfterFunc(time.Duration(config.DoorControl.ActivationPeriodMillis)*time.Millisecond, func() {
					log.Print("Requesting circuit deactivation")
					doorControl.doorControlRequest <- IDLE
				})
			default:
				log.Printf("Ignoring request for unknown state %s", req)
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
	d.doorControlRequest <- CIRCUIT_ACTIVE
	http.Redirect(w, &r.Request, "", http.StatusFound)
}
