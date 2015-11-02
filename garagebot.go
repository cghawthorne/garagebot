package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"net/http"
	"os"
)

var (
	pin = rpio.Pin(23)
)

func main() {
	// Open and map memory to access gpio, check for errors
	if err := rpio.Open(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Pull up pin
	pin.PullUp()

	// Unmap gpio memory when done
	defer rpio.Close()

	http.HandleFunc("/", handler)
	http.ListenAndServe(":80", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	doorStatus := pin.Read()
	var doorString string
	if doorStatus == 0 {
		doorString = "closed"
	} else {
		doorString = "open"
	}

	fmt.Fprintln(w, "Garage door is:", doorString)
}
