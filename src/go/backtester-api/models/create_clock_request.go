package models

type CreateClockRequest struct {
	StartDate string `json:"start"`
	StopDate  string `json:"stop"`
}
