package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
)

type SubmitTaskPayload struct {
	ProblemStmt string   `json:"problem_stmt"`
	GithubLink  string   `json:"github_link"`
	FigmaLink   string   `json:"figma_link"`
	OtherFiles  []string `json:"other_files"`
}

type UpdateTaskPayload struct {
	SubmissionID string   `json:"submission_id"` // Firestore document ID
	ProblemStmt  string   `json:"problem_stmt"`
	GithubLink   string   `json:"github_link"`
	FigmaLink    string   `json:"figma_link"`
	OtherFiles   []string `json:"other_files"`
}

func SubmitTasks(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := token.UID

	var payload SubmitTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	tasks := config.FirestoreClient.Collection("tasks")

	doc := map[string]interface{}{
		"user_id":      userID,
		"problem_stmt": payload.ProblemStmt,
		"github_link":  payload.GithubLink,
		"figma_link":   payload.FigmaLink,
		"other_files":  payload.OtherFiles,
		"submitted_at": time.Now(),
	}

	docRef, _, err := tasks.Add(ctx, doc)
	if err != nil {
		http.Error(w, "Failed to submit task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":       "Task submitted successfully",
		"submission_id": docRef.ID,
	})
}

func UpdateSubmission(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := token.UID

	var payload UpdateTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.SubmissionID == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	taskRef := config.FirestoreClient.Collection("tasks").Doc(payload.SubmissionID)

	docSnap, err := taskRef.Get(ctx)
	if err != nil || docSnap.Data()["user_id"] != userID {
		http.Error(w, "Submission not found or forbidden", http.StatusForbidden)
		return
	}

	updates := map[string]interface{}{
		"problem_stmt": payload.ProblemStmt,
		"github_link":  payload.GithubLink,
		"figma_link":   payload.FigmaLink,
		"other_files":  payload.OtherFiles,
		"updated_at":   time.Now(),
	}

	_, err = taskRef.Set(ctx, updates, firestore.MergeAll)
	if err != nil {
		http.Error(w, "Failed to update submission", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Submission updated successfully",
	})
}

//after user setup, will make sure only leader gets to upload
