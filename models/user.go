package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id" firestore:"-"` //mail taken from firestore
	RollNo    string    `gorm:"unique;not null" json:"rollNo"` //VIT
	Name      string    `gorm:"not null" json:"name"` //VIT
	Email     string    `gorm:"unique;not null" json:"email"` //VIT
	TeamID    *string   `json:"teamId"` //init - null -> join/leave -> gets changed to team id or null again
	IsLead    bool      `gorm:"default:false" json:"isLead"` //init - false -> create team -> true, leave team -> false
	PhoneNo   string    `json:"phoneNo"` //VIT
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
