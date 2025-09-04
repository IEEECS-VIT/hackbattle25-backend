package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"log"

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
	ProblemStmt  *string   `json:"problem_stmt"`
	GithubLink   *string   `json:"github_link"`
	FigmaLink    *string   `json:"figma_link"`
	OtherFiles   []string `json:"other_files"`
}

func verifyTeamLeader(ctx context.Context, userID string) (string, error) {
    userDoc, err := config.FirestoreClient.Collection("users").Doc(userID).Get(ctx)
    if err != nil {
        return "", &httpError{"User not found", http.StatusNotFound}
    }
    teamID, err := userDoc.DataAt("TeamID")
    if err != nil || teamID == nil {
        return "", &httpError{"User is not on a team", http.StatusForbidden}
    }

    teamDoc, err := config.FirestoreClient.Collection("teams").Doc(teamID.(string)).Get(ctx)
    if err != nil {
        return "", &httpError{"Team not found", http.StatusInternalServerError}
    }
    leaderID, err := teamDoc.DataAt("leaderId")
    if err != nil || leaderID.(string) != userID {
        return "", &httpError{"Only the team leader can perform this action", http.StatusForbidden}
    }

    return teamID.(string), nil
}

type httpError struct {
    message string
    code    int
}

func (e *httpError) Error() string {
    return e.message
}

func handleFirestoreError(w http.ResponseWriter, err error) {
    if e, ok := err.(*httpError); ok {
        http.Error(w, e.message, e.code)
    } else {
        // Log the actual error for debugging
        log.Printf("Internal server error: %v", err)
        http.Error(w, "Internal server error", http.StatusInternalServerError)
    }
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
	teamID, err := verifyTeamLeader(ctx, userID)
	if err != nil {
        handleFirestoreError(w, err)
        return
    }

	tasksCollection := config.FirestoreClient.Collection("tasks")
	iter := tasksCollection.Where("team_id", "==", teamID).Limit(1).Documents(ctx)
	if _, err := iter.Next(); err == nil {
        http.Error(w, "Team has already submitted. Use the update endpoint.", http.StatusConflict)
        return
    }

	doc := map[string]interface{}{
		"team_id":      teamID,
		"problem_stmt": payload.ProblemStmt,
		"github_link":  payload.GithubLink,
		"figma_link":   payload.FigmaLink,
		"other_files":  payload.OtherFiles,
		"submitted_at": time.Now(),
	}

	docRef, _, err := tasksCollection.Add(ctx, doc)
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
	ctx := context.Background()

	teamID, err := verifyTeamLeader(ctx, userID)
    if err != nil {
        handleFirestoreError(w, err)
        return
    }

	tasksCollection := config.FirestoreClient.Collection("tasks")
	iter := tasksCollection.Where("team_id", "==", teamID).Limit(1).Documents(ctx)
	docSnap, err := iter.Next()
	if err != nil {
		http.Error(w, "Submission not found for this team", http.StatusForbidden)
		return
	}
	taskRef := docSnap.Ref

	var payload UpdateTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	 var updates []firestore.Update

    if payload.ProblemStmt != nil {
        updates = append(updates, firestore.Update{Path: "problem_stmt", Value: *payload.ProblemStmt})
    }
    if payload.GithubLink != nil {
        updates = append(updates, firestore.Update{Path: "github_link", Value: *payload.GithubLink})
    }
    if payload.FigmaLink != nil {
        updates = append(updates, firestore.Update{Path: "figma_link", Value: *payload.FigmaLink})
    }
    if payload.OtherFiles != nil {
        updates = append(updates, firestore.Update{Path: "other_files", Value: payload.OtherFiles})
    }

    if len(updates) == 0 {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "No fields to update"})
        return
    }

    updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})

	_, err = taskRef.Update(ctx, updates)
	if err != nil {
		http.Error(w, "Failed to update submission", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Submission updated successfully",
	})
}
