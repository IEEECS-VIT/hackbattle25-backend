package models

import "time"

type Submission struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TeamID      uint      `gorm:"unique;not null" json:"teamId"`    
	SubmittedBy uint      `gorm:"not null" json:"submittedBy"` //team leader
	IdeaTitle   string    `gorm:"not null" json:"ideaTitle"`
	Description string    `json:"description"`
	GithubLink  *string   `json:"githubLink,omitempty"`
	FigmaLink   *string   `json:"figmaLink,omitempty"`
	FileLink    *string   `json:"fileLink,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}