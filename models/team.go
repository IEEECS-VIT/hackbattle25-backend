package models

import "time"

type Team struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Code      string    `gorm:"unique;not null;size:6" json:"code"`
	CreatedAt time.Time `json:"createdAt"`

	Users []User `gorm:"foreignKey:TeamID" json:"users,omitempty"`

	// Submission fields are now embedded directly into the Team model.
	// Using pointers makes them optional, representing a non-existent submission.
	ProblemStmt *string    `json:"problem_stmt,omitempty"`
	GithubLink  *string    `json:"github_link,omitempty"`
	FigmaLink   *string    `json:"figma_link,omitempty"`
	OtherFiles  []string   `json:"other_files,omitempty"` // Slices are reference types, so omitempty works well.
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}
