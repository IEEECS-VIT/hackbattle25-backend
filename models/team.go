package models

import "time"

type Team struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Code      string    `gorm:"unique;not null;size:6" json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	LeaderID string    `json:"leader_id"`
	Users []User `gorm:"foreignKey:TeamID" json:"users,omitempty"`
	ProblemStmt *string    `json:"problem_stmt,omitempty"`
	GithubLink  *string    `json:"github_link,omitempty"`
	FigmaLink   *string    `json:"figma_link,omitempty"`
	OtherFiles  []string   `json:"other_files,omitempty"` 
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}
