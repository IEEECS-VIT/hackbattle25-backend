package models

import (
	"time"
)

type Team struct {
	ID          string     `json:"id" firestore:"-"`
	Name        string     `json:"name" firestore:"Name"`
	Code        string     `json:"code" firestore:"Code"`
	CreatedAt   time.Time  `json:"createdAt" firestore:"CreatedAt"`
	LeaderID    string     `json:"leader_id" firestore:"leaderId"`

	// Track & Subtrack Fields
	Track       *string    `json:"track,omitempty" firestore:"Track,omitempty"`
	Subtrack    *string    `json:"subtrack,omitempty" firestore:"Subtrack,omitempty"`

	// Submission Details
	ProjectDesc *string    `json:"project_desc,omitempty" firestore:"ProjectDesc,omitempty"`
	GithubLink  *string    `json:"github_link,omitempty" firestore:"GithubLink,omitempty"`
	FigmaLink   *string    `json:"figma_link,omitempty" firestore:"FigmaLink,omitempty"`
	OtherFiles  *string    `json:"other_files,omitempty" firestore:"OtherFiles,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty" firestore:"SubmittedAt,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" firestore:"UpdatedAt,omitempty"`
}