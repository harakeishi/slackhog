package main

import "time"

// Message はSlackから受信したメッセージを表す。
type Message struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Username    string    `json:"username"`
	Text        string    `json:"text"`
	IconEmoji   string    `json:"icon_emoji,omitempty"`
	IconURL     string    `json:"icon_url,omitempty"`
	Blocks      any       `json:"blocks,omitempty"`
	Attachments any       `json:"attachments,omitempty"`
	ReceivedAt  time.Time `json:"received_at"`
	RawPayload  any       `json:"raw_payload"`
}
