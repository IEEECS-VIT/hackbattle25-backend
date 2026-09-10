package models

import "time"

type User struct {
	Name               string    `json:"name" firestore:"name"`
	Email              string    `json:"email" firestore:"email"`
	TeamID             *string   `json:"teamId,omitempty" firestore:"TeamID,omitempty"`
	IsLead             bool      `json:"isLead" firestore:"IsLead"`
	IsVITian           bool    `json:"isVITian" firestore:"isVITian"`
	RegNo             string    `json:"regNo" firestore:"regNo"`
	CreatedAt          time.Time `json:"createdAt" firestore:"createdAt"`
}