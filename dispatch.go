package main

import (
	"log"
)

type Dispatcher struct {
	sender                    StatusUpdateChan
	listeners                 []StatusUpdateChan
	createListenerRequestChan chan *CreateListenerRequest
}

type CreateListenerRequest struct {
	resultChan chan StatusUpdateChan
}

func createDispatcher(statusUpdates StatusUpdateChan) *Dispatcher {
	dispatcher := &Dispatcher{statusUpdates, make([]StatusUpdateChan, 0), make(chan *CreateListenerRequest)}

	go func() {
		for {
			select {
			case req := <-dispatcher.createListenerRequestChan:
				log.Print("Got create listener request!")
				listener := make(StatusUpdateChan, 1)
				dispatcher.listeners = append(dispatcher.listeners, listener)
				req.resultChan <- listener
			case update := <-dispatcher.sender:
				log.Print("Got update!")
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
