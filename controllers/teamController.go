package controllers

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
	"github.com/IEEECS-VIT/hackbattle25-backend/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TeamPayload struct {
	Name string `json:"name"`
}

const maxTeamSize = 5

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
		return
	}
	st := status.Convert(err)
	switch st.Code() {
	case codes.NotFound:
		http.Error(w, st.Message(), http.StatusNotFound)
	case codes.AlreadyExists:
		http.Error(w, st.Message(), http.StatusConflict)
	case codes.FailedPrecondition:
		http.Error(w, st.Message(), http.StatusForbidden)
	default:
		http.Error(w, "An unexpected error occurred: "+st.Message(), http.StatusInternalServerError)
	}
}

func verifyTeamLeader(ctx context.Context, leaderID string) (string, error) {
	userDoc, err := config.FirestoreClient.Collection("users").Doc(leaderID).Get(ctx)
	if err != nil {
		return "", &httpError{"User profile not found", http.StatusNotFound}
	}

	isLead, err := userDoc.DataAt("IsLead")
	if err != nil || !isLead.(bool) {
		return "", &httpError{"User is not a team leader", http.StatusForbidden}
	}

	teamID, err := userDoc.DataAt("TeamID")
	if err != nil {
		return "", &httpError{"Team ID not found for leader", http.StatusInternalServerError}
	}

	return teamID.(string), nil
}

func generateTeamCode() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	var sb strings.Builder
	for i := 0; i < 6; i++ {
		sb.WriteByte(letters[rand.Intn(len(letters))])
	}
	return sb.String()
}

func CreateTeam(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	userEmail := token.Claims["email"].(string)

	var payload TeamPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		http.Error(w, "Invalid team name", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userID)
	teamsCollection := config.FirestoreClient.Collection("teams")

	// Check for duplicate team name
	q := teamsCollection.Where("Name", "==", payload.Name).Limit(1)
	if docs, _ := q.Documents(ctx).GetAll(); len(docs) > 0 {
		http.Error(w, "This team name is already taken", http.StatusConflict)
		return
	}

	teamCode := generateTeamCode()
	newTeamRef := teamsCollection.Doc(teamCode) // Use generated code as document ID

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}
		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			return status.Errorf(codes.AlreadyExists, "User is already in a team")
		}

		err = tx.Set(newTeamRef, map[string]interface{}{
			"Name":      payload.Name,
			"Code":      teamCode,
			"leaderId":  userID,
			"members":   []string{userID},
			"emails":    []string{userEmail},
			"CreatedAt": time.Now(),
		})
		if err != nil {
			return err
		}

		return tx.Update(userRef, []firestore.Update{
			{Path: "TeamID", Value: newTeamRef.ID},
			{Path: "IsLead", Value: true},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Team created successfully",
		"code":    teamCode,
	})
}

func JoinTeam(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	userEmail := token.Claims["email"].(string)

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamCode == "" {
		http.Error(w, "Invalid team code provided", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userID)
	teamRef := config.FirestoreClient.Collection("teams").Doc(req.TeamCode)

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}
		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			return status.Errorf(codes.AlreadyExists, "User is already in a team")
		}

		teamSnap, err := tx.Get(teamRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return status.Errorf(codes.NotFound, "Team with that code not found")
			}
			return err
		}

		members, _ := teamSnap.DataAt("members")
		if len(members.([]interface{})) >= maxTeamSize {
			return status.Errorf(codes.FailedPrecondition, "Team is already full")
		}

		updates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayUnion(userID)},
			{Path: "emails", Value: firestore.ArrayUnion(userEmail)},
		}
		if err := tx.Update(teamRef, updates); err != nil {
			return err
		}
		return tx.Update(userRef, []firestore.Update{{Path: "TeamID", Value: teamRef.ID}})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "User joined team successfully"})
}

func LeaveTeam(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	userEmail := token.Claims["email"].(string)
	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userID)

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}

		teamIDValue, err := userDoc.DataAt("TeamID")
		if err != nil || teamIDValue == nil {
			return status.Errorf(codes.FailedPrecondition, "User is not in a team")
		}
		teamID := teamIDValue.(string)

		isLead, _ := userDoc.DataAt("IsLead")
		if isLead.(bool) {
			return status.Errorf(codes.FailedPrecondition, "Leaders cannot leave a team. You must delete the team or transfer leadership.")
		}

		teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
		updates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(userID)},
			{Path: "emails", Value: firestore.ArrayRemove(userEmail)},
		}
		if err := tx.Update(teamRef, updates); err != nil {
			return err
		}
		return tx.Update(userRef, []firestore.Update{
			{Path: "TeamID", Value: nil},
			{Path: "IsLead", Value: false},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully left team"})
}

func GetTeam(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	ctx := context.Background()

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamIDValue, err := userDoc.DataAt("TeamID")
	if err != nil || teamIDValue == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "User is not part of any team"})
		return
	}
	teamID := teamIDValue.(string)

	teamDoc, err := config.FirestoreClient.Collection("teams").Doc(teamID).Get(ctx)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	var teamData models.Team
	if err := teamDoc.DataTo(&teamData); err != nil {
		http.Error(w, "Failed to parse team data", http.StatusInternalServerError)
		return
	}

	type MemberDetails struct {
		UID   string `json:"uid"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	memberDetailsList := make([]MemberDetails, 0)
	teamMembers, err := teamDoc.DataAt("members")
	if err == nil {
		for _, memberUID := range teamMembers.([]interface{}) {
			uidStr := memberUID.(string)
			memberDoc, err := config.FirestoreClient.Collection("users").Doc(uidStr).Get(ctx)
			if err == nil {
				var name, email string
				if n, ok := memberDoc.Data()["Name"].(string); ok {
					name = n
				}
				if e, ok := memberDoc.Data()["Email"].(string); ok {
					email = e
				}
				memberDetailsList = append(memberDetailsList, MemberDetails{
					UID:   uidStr,
					Name:  name,
					Email: email,
				})
			}
		}
	}

	leaderID, _ := teamDoc.DataAt("leaderId")

	response := map[string]interface{} {
		"id":           teamDoc.Ref.ID,
		"name":         teamData.Name,
		"code":         teamData.Code,
		"leaderId":     leaderID,
		"members":      memberDetailsList,
		"problem_stmt": teamData.ProblemStmt,
		"github_link":  teamData.GithubLink,
		"figma_link":   teamData.FigmaLink,
		"other_files":  teamData.OtherFiles,
		"submitted_at": teamData.SubmittedAt,
		"updated_at":   teamData.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func ChangeTeamName(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	ctx := context.Background()

	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		http.Error(w, "Invalid team name provided", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, userID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	q := config.FirestoreClient.Collection("teams").Where("Name", "==", payload.Name).Limit(1)
	if docs, _ := q.Documents(ctx).GetAll(); len(docs) > 0 {
		http.Error(w, "This team name is already taken", http.StatusConflict)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	_, err = teamRef.Update(ctx, []firestore.Update{{Path: "Name", Value: payload.Name}})
	if err != nil {
		http.Error(w, "Failed to update team name", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Team name updated successfully"})
}

func RemoveMember(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	leaderID := token.UID
	ctx := context.Background()

	var payload struct {
		MemberID string `json:"memberId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.MemberID == "" {
		http.Error(w, "Invalid member ID provided", http.StatusBadRequest)
		return
	}

	if leaderID == payload.MemberID {
		http.Error(w, "Leader cannot remove themselves from the team", http.StatusForbidden)
		return
	}

	teamID, err := verifyTeamLeader(ctx, leaderID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	memberRef := config.FirestoreClient.Collection("users").Doc(payload.MemberID)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		memberDoc, err := tx.Get(memberRef)
		if err != nil {
			return &httpError{"Member user profile not found", http.StatusNotFound}
		}
		memberTeamID, _ := memberDoc.DataAt("TeamID")
		if memberTeamID != teamID {
			return &httpError{"This member is not part of your team", http.StatusBadRequest}
		}

		memberEmail, _ := memberDoc.DataAt("Email")

		updates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(payload.MemberID)},
			{Path: "emails", Value: firestore.ArrayRemove(memberEmail)},
		}
		if err := tx.Update(teamRef, updates); err != nil {
			return err
		}

		return tx.Update(memberRef, []firestore.Update{
			{Path: "TeamID", Value: nil},
			{Path: "IsLead", Value: false},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Member removed successfully"})
}

func DeleteTeam(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	leaderID := token.UID
	ctx := context.Background()

	teamID, err := verifyTeamLeader(ctx, leaderID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	leaderRef := config.FirestoreClient.Collection("users").Doc(leaderID)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		teamDoc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}

		members, err := teamDoc.DataAt("members")
		if err != nil {
			return err
		}

		if len(members.([]interface{})) > 1 {
			return &httpError{"You must remove all other members before deleting the team", http.StatusForbidden}
		}

		if err := tx.Delete(teamRef); err != nil {
			return err
		}

		return tx.Update(leaderRef, []firestore.Update{
			{Path: "TeamID", Value: nil},
			{Path: "IsLead", Value: false},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Team deleted successfully"})
}

// SubmissionPayload defines the structure for project submission.
type SubmissionPayload struct {
	ProblemStmt *string  `json:"problem_stmt"`
	GithubLink  *string  `json:"github_link"`
	FigmaLink   *string  `json:"figma_link"`
	OtherFiles  []string `json:"other_files"`
}

func SubmitProject(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	ctx := context.Background()

	var payload SubmissionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if payload.ProblemStmt == nil || *payload.ProblemStmt == "" || payload.GithubLink == nil || *payload.GithubLink == "" {
		http.Error(w, "Problem statement and GitHub link are required", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, userID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}
		if _, err := doc.DataAt("SubmittedAt"); err == nil {
			return &httpError{"Project has already been submitted", http.StatusConflict}
		}

		now := time.Now()
		return tx.Update(teamRef, []firestore.Update{
			{Path: "ProblemStmt", Value: payload.ProblemStmt},
			{Path: "GithubLink", Value: payload.GithubLink},
			{Path: "FigmaLink", Value: payload.FigmaLink},
			{Path: "OtherFiles", Value: payload.OtherFiles},
			{Path: "SubmittedAt", Value: now},
			{Path: "UpdatedAt", Value: now},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Project submitted successfully"})
}

func UpdateProject(w http.ResponseWriter, r *http.Request) {
	token := r.Context().Value(middleware.UserKey).(*auth.Token)
	userID := token.UID
	ctx := context.Background()

	var payload SubmissionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, userID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}
		if _, err := doc.DataAt("SubmittedAt"); err != nil {
			return &httpError{"Project has not been submitted yet. Use the submit endpoint first.", http.StatusForbidden}
		}

		updates := []firestore.Update{}
		if payload.ProblemStmt != nil {
			updates = append(updates, firestore.Update{Path: "ProblemStmt", Value: *payload.ProblemStmt})
		}
		if payload.GithubLink != nil {
			updates = append(updates, firestore.Update{Path: "GithubLink", Value: *payload.GithubLink})
		}
		if payload.FigmaLink != nil {
			updates = append(updates, firestore.Update{Path: "FigmaLink", Value: *payload.FigmaLink})
		}
		if payload.OtherFiles != nil {
			updates = append(updates, firestore.Update{Path: "OtherFiles", Value: payload.OtherFiles})
		}

		if len(updates) == 0 {
			return &httpError{"No update data provided", http.StatusBadRequest}
		}

		updates = append(updates, firestore.Update{Path: "UpdatedAt", Value: time.Now()})

		return tx.Update(teamRef, updates)
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Project updated successfully"})
}