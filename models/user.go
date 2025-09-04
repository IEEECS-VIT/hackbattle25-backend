package models
import "time"

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id" firestore:"-"`
	Name         string     `gorm:"not null" json:"name"`
	Email        string     `gorm:"unique;not null" json:"email"`
	TeamID       *string    `json:"teamId"`
	IsLead       bool       `gorm:"default:false" json:"isLead"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}