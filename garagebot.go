package main

import (
	"fmt"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
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

	// Unmap gpio memory when done
	defer rpio.Close()

	// Pull up and read value
	pin.PullUp()

	for {
		fmt.Printf("PullUp: %d\n", pin.Read())
		time.Sleep(1 * time.Second)
	}
}
