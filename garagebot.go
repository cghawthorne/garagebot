package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"net/http"
)

type DoorStatus int

const (
	OPEN DoorStatus = iota
	CLOSED
)

type StatusRequest struct {
	resultChan chan DoorStatus
}

func main() {
	statusChan := make(chan *StatusRequest)
	go doorMonitor(statusChan)

	http.Handle("/", &StatusPage{statusChan})
	http.ListenAndServe(":80", nil)
}

func doorMonitor(queue chan *StatusRequest) {
	// Open and map memory to access gpio, check for errors.
	if err := rpio.Open(); err != nil {
		panic(err)
	}
	// Unmap gpio memory when done.
	defer rpio.Close()

	pin := rpio.Pin(23)
	pin.PullUp()

	// Process request queue.
	for req := range queue {
		doorStatus := pin.Read()
		if doorStatus == 0 {
			req.resultChan <- CLOSED
		} else {
			req.resultChan <- OPEN
		}
	}
}

type StatusPage struct {
	statusChan chan *StatusRequest
}

func (s *StatusPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := &StatusRequest{make(chan DoorStatus)}
	s.statusChan <- req
	doorStatus := <-req.resultChan
	var doorString string
	if doorStatus == CLOSED {
		doorString = "closed"
	} else {
		doorString = "open"
	}

	fmt.Fprintln(w, "Garage door is:", doorString)
}
