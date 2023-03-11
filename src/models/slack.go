package models

import "fmt"

type IncomingSlackMessage struct {
	Text     string
	Channel  string
	FromUser string
	ToUser   string
}

type SlackVerificationRequest struct {
	Challenge string `json:"challenge"`
	Token     string `json:"token"`
	Type      string `json:"type"`
}

type SlackEvent map[string]interface{}

func (ev SlackEvent) GetType() interface{} {
	if ev["type"] == "url_verification" {
		return &SlackVerificationRequest{
			Challenge: ev["challenge"].(string),
			Token:     ev["token"].(string),
			Type:      "url_verification",
		}
	}

	if ev["type"] == "event_callback" {
		event := ev["event"].(map[string]interface{})
		slackMessage, ok := event["text"].(string)
		if ok {
			channel, hasChannel := event["channel"]
			if !hasChannel {
				fmt.Printf("WARNING: could not locate channel for %v\n.", ev)
				return nil
			}

			// Important: this block prevents the bot from looping when it receives it\'s own message
			fromUser, hasFromUser := event["user"]
			if !hasFromUser {
				// fmt.Printf("WARNING: could not locate user for %v\n. This is known to happen when bots send messages.", ev)
				return nil
			}

			return &IncomingSlackMessage{
				Text:     slackMessage,
				Channel:  channel.(string),
				FromUser: fromUser.(string),
			}
		}
	}

	return nil
}
