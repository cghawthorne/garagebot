package main

import (
	"fmt"
	"log"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	NOTIFICATION_STATUS       = OPEN
	NOTIFICATION_CHECK_PERIOD = time.Duration(1) * time.Second
)

func sendNotificationEmail(config *Configuration, status string) {
	to := strings.Join(config.Notifications.Emails, ",")
	msg := "To: " + to + "\r\n" +
		"Subject: " + status + "\r\n" +
		"\r\n" +
		status

	log.Print("Sending email: ", msg)

	auth := smtp.PlainAuth("",
		config.Notifications.From, config.Notifications.Password, config.Notifications.Server)
	err := smtp.SendMail(config.Notifications.Server+":"+strconv.Itoa(config.Notifications.Port), auth,
		config.Notifications.From, config.Notifications.Emails, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %v", err)
	}
}

func notifier(config *Configuration, statusUpdates StatusUpdateChan) {
	statusChange := time.Now()
	lastStatus := STARTUP
	openAlertSent := false

	timeout := time.Duration(config.Notifications.TimeoutMillis) * time.Millisecond

	ticker := time.NewTicker(NOTIFICATION_CHECK_PERIOD)
	for {
		select {
		case <-ticker.C:
			if !openAlertSent && lastStatus == NOTIFICATION_STATUS && time.Since(statusChange) > timeout {
				status := fmt.Sprintf("Door has been %v for %v", lastStatus.String(), timeout)
				sendNotificationEmail(config, status)
				openAlertSent = true
			}
		case update := <-statusUpdates:
			log.Print("Notifier got update:", update)
			if update == CLOSED && openAlertSent {
				sendNotificationEmail(config, "Door has closed")
			}
			if update != NOTIFICATION_STATUS {
				openAlertSent = false
			}
			lastStatus = update
			statusChange = time.Now()
		}
	}
}
