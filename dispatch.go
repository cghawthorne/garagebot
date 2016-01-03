package main

import (
	"log"
)

type Dispatcher struct {
	input                       StatusUpdateChan
	listeners                   []StatusUpdateChan
	createListenerRequestChan   chan *CreateListenerRequest
	startDispatchingRequestChan chan bool
}

type CreateListenerRequest struct {
	resultChan chan StatusUpdateChan
}

func createDispatcher(statusUpdates StatusUpdateChan) *Dispatcher {
	dispatcher := &Dispatcher{
		statusUpdates,
		make([]StatusUpdateChan, 0),
		make(chan *CreateListenerRequest),
		make(chan bool),
	}

	go func() {
		log.Print("Dispatcher ready to add listeners")
	ListenerLoop:
		for {
			select {
			case req := <-dispatcher.createListenerRequestChan:
				log.Print("Got create listener request!")
				listener := make(StatusUpdateChan, 1)
				dispatcher.listeners = append(dispatcher.listeners, listener)
				req.resultChan <- listener
			case <-dispatcher.startDispatchingRequestChan:
				break ListenerLoop
			}
		}
		log.Print("Dispatcher done adding listeners. Entering dispatch mode.")
		for {
			select {
			case <-dispatcher.createListenerRequestChan:
				log.Print("Got create listener request, but already in dispatch mode! Ignoring!")
			case <-dispatcher.startDispatchingRequestChan:
				log.Print("Got request to enter dispatch mode, but already in dispatch mode! Ignoring!")
			case update := <-dispatcher.input:
				log.Printf("Got update:%s", update)
				for _, listener := range dispatcher.listeners {
					listener <- update
				}
			}
		}
	}()

	return dispatcher
}

func (d *Dispatcher) createListener() StatusUpdateChan {
	log.Print("Creating listener")
	req := &CreateListenerRequest{make(chan StatusUpdateChan)}
	d.createListenerRequestChan <- req
	return <-req.resultChan
}

func (d *Dispatcher) startDispatch() {
	log.Print("Requesting dispatch mode")
	d.startDispatchingRequestChan <- true
}
