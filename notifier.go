package main

import (
	"log"
	"strings"
	"time"
)

const (
	NOTIFICATION_STATUS       = CLOSED
	NOTIFICATION_CHECK_PERIOD = time.Duration(1) * time.Second
)

func notifier(config *Configuration, statusUpdates StatusUpdateChan) {
	statusChange := time.Now()
	lastStatus := STARTUP
	notificationSent := false

	timeout := time.Duration(config.Notifications.TimeoutMillis) * time.Millisecond

	ticker := time.NewTicker(NOTIFICATION_CHECK_PERIOD)
	for {
		select {
		case <-ticker.C:
			if !notificationSent && lastStatus == NOTIFICATION_STATUS && time.Since(statusChange) > timeout {
				log.Printf("Timeout (%v) expired. Sending notifications to: %v",
					timeout, strings.Join(config.Notifications.Emails, ", "))
				// TODO: send notifications
				notificationSent = true
			}
		case update := <-statusUpdates:
			log.Print("Notifier got update:", update)
			lastStatus = update
			statusChange = time.Now()
			notificationSent = false
		}
	}
}
