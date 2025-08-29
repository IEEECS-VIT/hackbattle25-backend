package models

import "time"

type Team struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Code      string    `gorm:"unique;not null;size:6" json:"code"`
	CreatedAt time.Time `json:"createdAt"`

	Users       []User      `gorm:"foreignKey:TeamID" json:"users,omitempty"`
	Submission  *Submission `gorm:"foreignKey:TeamID" json:"submission,omitempty"`
}
