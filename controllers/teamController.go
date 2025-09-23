package controllers

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"log"
	
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

func getUserEmailFromContext(r *http.Request) (string, bool) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		return "", false
	}
	email, ok := token.Claims["email"].(string)
	if !ok {
		return "", false
	}
	return strings.ToLower(email), true
}

func verifyTeamLeader(ctx context.Context, r *http.Request) (string, error) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		return "", &httpError{"Invalid token: missing email", http.StatusUnauthorized}
	}

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userEmail).Get(ctx)
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

func generateTeamCode(ctx context.Context, teamsCollection *firestore.CollectionRef) (string, error) {
	for {
		code := randomTeamCode()
		doc, err := teamsCollection.Doc(code).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return code, nil
			}
			return "", err
		}
		if !doc.Exists() {
			return code, nil
		}
	}
}

func randomTeamCode() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func getUserNameFromContext(r *http.Request) (string, bool) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		return "", false
	}
	name, ok := token.Claims["name"].(string)
	return name, ok
}

func CreateTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}

	userName, ok := getUserNameFromContext(r)
	if !ok || userName == "" {
		http.Error(w, "Invalid token: missing display name", http.StatusUnauthorized)
		return
	}

	var payload TeamPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		http.Error(w, "Invalid team name", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
	teamsCollection := config.FirestoreClient.Collection("teams")

	// Check if team name already exists
	q := teamsCollection.Where("Name", "==", payload.Name).Limit(1)
	if docs, _ := q.Documents(ctx).GetAll(); len(docs) > 0 {
		http.Error(w, "This team name is already taken", http.StatusConflict)
		return
	}

	teamCode, _ := generateTeamCode(ctx, teamsCollection)

	var alreadyInTeam bool

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}

		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			alreadyInTeam = true
			return nil // stop here, don’t create a team
		}

		newTeamRef := teamsCollection.Doc(teamCode)
		if err := tx.Set(newTeamRef, map[string]interface{}{
			"Name":      payload.Name,
			"Code":      teamCode,
			"leaderId":  userEmail,
			"members":   []map[string]interface{}{{"email": userEmail, "name": userName}},
			"CreatedAt": time.Now(),
		}); err != nil {
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

	if alreadyInTeam {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "You are already in a team"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Team created successfully",
		"code":    teamCode,
	})
}


// JoinTeam adds email+name as a member
func JoinTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamCode == "" {
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid or missing team code"})
		return
	}

	req.TeamCode = strings.ToUpper(req.TeamCode)
	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
	teamRef := config.FirestoreClient.Collection("teams").Doc(req.TeamCode)

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}
		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"message": "You are already in a team"})
		}

		teamSnap, err := tx.Get(teamRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				w.WriteHeader(http.StatusNoContent)
				json.NewEncoder(w).Encode(map[string]string{"message": "Team with that code not found"})
			}
			return err
		}

		members, _ := teamSnap.DataAt("members")
		if len(members.([]interface{})) >= maxTeamSize {
			w.WriteHeader(http.StatusAlreadyReported)
			json.NewEncoder(w).Encode(map[string]string{"message": "Team at max size"})	
		}

		userName, ok := getUserNameFromContext(r)
		if !ok || userName == "" {
			return status.Errorf(codes.InvalidArgument, "Missing user name")
		}
		updates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayUnion(map[string]interface{}{"email": userEmail, "name": userName})},
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

func LeaveOrDeleteTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
	var teamID string

	// Use a transaction to ensure atomicity
	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}

		// Get user's TeamID and check if they are in a team
		teamIDData, _ := userDoc.DataAt("TeamID")
		if teamIDData == nil {
			return status.Errorf(codes.FailedPrecondition, "User is not in a team")
		}

		// safely extract teamID
		switch v := teamIDData.(type) {
		case string:
			teamID = v
		case *firestore.DocumentRef:
			teamID = v.ID
		default:
			return status.Errorf(codes.Internal, "TeamID field is invalid")
		}


		// Get user's IsLead status
		IsLeadData, _ := userDoc.DataAt("IsLead")
		IsLead := false
		if lead, ok := IsLeadData.(bool); ok {
			IsLead = lead
		}

		teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
		teamDoc, err := tx.Get(teamRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "Team not found")
		}
		membersData, _ := teamDoc.DataAt("members")
		members := membersData.([]interface{})

		// Get user's name from their user document for the ArrayRemove operation
		userName := userDoc.Data()["name"]
		if userName == nil {
			// Handle case where user name is missing, though this shouldn't happen
			return status.Errorf(codes.Internal, "User name not found in user document")
		}
		log.Printf("IsLead: %v\n", IsLead)
		// Logic for a Team Leader
		if IsLead {
			if len(members) > 1 {
				// Transfer leadership to the next member
				var newLeadEmail string
				for _, member := range members {
					memberMap := member.(map[string]interface{})
					if memberMap["email"].(string) != userEmail {
						newLeadEmail = memberMap["email"].(string)
						break
					}
				}

				if newLeadEmail == "" {
					return status.Errorf(codes.Internal, "Could not find a new leader.")
				}

				newLeaderRef := config.FirestoreClient.Collection("users").Doc(newLeadEmail)
				if err := tx.Update(newLeaderRef, []firestore.Update{{Path: "IsLead", Value: true}}); err != nil {
					return err
				}

				if err := tx.Update(teamRef, []firestore.Update{{Path: "leaderId", Value: newLeadEmail}}); err != nil {
					return err
				}

				// Remove leader from team members list
				// Remove leaving leader manually
				newMembers := []interface{}{}
				for _, member := range members {
					memberMap := member.(map[string]interface{})
					if memberMap["email"].(string) != userEmail {
						newMembers = append(newMembers, memberMap)
					}
				}

				if err := tx.Update(teamRef, []firestore.Update{
					{Path: "members", Value: newMembers},
				}); err != nil {
					return err
				}


				// Update the original leader's user document
				return tx.Update(userRef, []firestore.Update{
					{Path: "TeamID", Value: firestore.Delete},
					{Path: "IsLead", Value: firestore.Delete},
				})
			} else {
				// The leader is the only one left, so delete the team
				if err := tx.Delete(teamRef); err != nil {
					return err
				}

				// Update the leader's user document
				return tx.Update(userRef, []firestore.Update{
					{Path: "TeamID", Value: firestore.Delete},
					{Path: "IsLead", Value: firestore.Delete},
				})
			}
		}

		// Logic for a regular Team Member
		// This is the code block that is executed when a regular member leaves.
		// Remove leaving user manually
		newMembers := []interface{}{}
		for _, member := range members {
			memberMap := member.(map[string]interface{})
			if memberMap["email"].(string) != userEmail {
				newMembers = append(newMembers, memberMap)
			}
		}
		if err := tx.Update(teamRef, []firestore.Update{
			{Path: "members", Value: newMembers},
		}); err != nil {
			return err
		}


		return tx.Update(userRef, []firestore.Update{
			{Path: "TeamID", Value: nil},
			{Path: "IsLead", Value: firestore.Delete},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Action completed successfully"})
}

// LeaveTeam removes email+name
func LeaveTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}

		teamID, _ := userDoc.DataAt("TeamID")
		if teamID == nil {
			return status.Errorf(codes.FailedPrecondition, "User is not in a team")
		}

		isLeadData, _ := userDoc.DataAt("IsLead")
		if isLead, ok := isLeadData.(bool); ok && isLead {
			return status.Errorf(codes.FailedPrecondition, "Leaders cannot leave a team. Delete team or transfer leadership.")
		}

		teamRef := config.FirestoreClient.Collection("teams").Doc(teamID.(string))
		teamUpdates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(map[string]interface{}{"email": userEmail, "name": userDoc.Data()["Name"]})},
		}
		if err := tx.Update(teamRef, teamUpdates); err != nil {
			return err
		}

		return tx.Update(userRef, []firestore.Update{
			{Path: "TeamID", Value: firestore.Delete},
			{Path: "IsLead", Value: firestore.Delete},
		})
	})

	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully left team"})
}

// RemoveMember removes by email+name
func RemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var payload struct {
		MemberEmail string `json:"memberEmail"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.MemberEmail == "" {
		http.Error(w, "Invalid member email provided", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, r)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	if payload.MemberEmail == "" {
		http.Error(w, "Member email required", http.StatusBadRequest)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	memberRef := config.FirestoreClient.Collection("users").Doc(payload.MemberEmail)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		memberDoc, err := tx.Get(memberRef)
		if err != nil {
			return &httpError{"Member user profile not found", http.StatusNotFound}
		}

		memberName, _ := memberDoc.DataAt("Name")

		teamUpdates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(map[string]interface{}{"email": payload.MemberEmail, "name": memberName})},
		}
		if err := tx.Update(teamRef, teamUpdates); err != nil {
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

// GetTeam returns members with name+email
func GetTeam(w http.ResponseWriter, r *http.Request) {
	log.Println("GetTeam called")

	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		log.Println("Failed to get user email from context")
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}
	log.Println("User email from context:", userEmail)

	ctx := context.Background()

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userEmail).Get(ctx)
	if err != nil {
		log.Println("Error fetching user document:", err)
		handleFirestoreError(w, err)
		return
	}
	log.Println("User document fetched successfully")

	teamIDValue, _ := userDoc.DataAt("TeamID")
	if teamIDValue == nil {
		log.Println("User is not part of any team")
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"message": "User is not part of any team"})
		return
	}
	teamID := teamIDValue.(string)
	log.Println("Team ID:", teamID)

	teamDoc, err := config.FirestoreClient.Collection("teams").Doc(teamID).Get(ctx)
	if err != nil {
		log.Println("Error fetching team document:", err)
		handleFirestoreError(w, err)
		return
	}
	log.Println("Team document fetched successfully")

	var teamData models.Team
	if err := teamDoc.DataTo(&teamData); err != nil {
		log.Println("Failed to parse team data:", err)
		http.Error(w, "Failed to parse team data", http.StatusInternalServerError)
		return
	}
	log.Println("Team data parsed successfully:", teamData)

	membersList, _ := teamDoc.DataAt("members")
	log.Println("Members list fetched:", membersList)

	response := map[string]interface{}{
		"id":           teamDoc.Ref.ID,
		"name":         teamData.Name,
		"code":         teamData.Code,
		"leaderId":     teamData.LeaderID,
		"members":      membersList,
		"problem_stmt": teamData.ProblemStmt,
		"github_link":  teamData.GithubLink,
		"figma_link":   teamData.FigmaLink,
		"other_files":  teamData.OtherFiles,
		"submitted_at": teamData.SubmittedAt,
		"updated_at":   teamData.UpdatedAt,
		"isLeader":    teamData.LeaderID == userEmail,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	log.Println("Response sent successfully")
}

// DeleteTeam clears members
func DeleteTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}
	ctx := context.Background()

	teamID, err := verifyTeamLeader(ctx, r)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	leaderRef := config.FirestoreClient.Collection("users").Doc(userEmail)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		teamDoc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}

		members, _ := teamDoc.DataAt("members")
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

type SubmissionPayload struct {
	ProblemStmt *string  `json:"problem_stmt"`
	GithubLink  *string  `json:"github_link"`
	FigmaLink   *string  `json:"figma_link"`
	OtherFiles  string `json:"other_files"`
}

func SubmitProject(w http.ResponseWriter, r *http.Request) {
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

    teamID, err := verifyTeamLeader(ctx, r)
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

        now := time.Now()
        updates := []firestore.Update{
            {Path: "ProblemStmt", Value: payload.ProblemStmt},
            {Path: "GithubLink", Value: payload.GithubLink},
            {Path: "FigmaLink", Value: payload.FigmaLink},
            {Path: "OtherFiles", Value: payload.OtherFiles},
            {Path: "UpdatedAt", Value: now},
        }

        // Only set SubmittedAt if it doesn't exist yet
        if _, err := doc.DataAt("SubmittedAt"); err != nil {
            updates = append(updates, firestore.Update{Path: "SubmittedAt", Value: now})
        }

        return tx.Update(teamRef, updates)
    })

    if err != nil {
        handleFirestoreError(w, err)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Project submitted/updated successfully"})
}

func UpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var payload SubmissionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, r)
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
		updates = append(updates, firestore.Update{Path: "OtherFiles", Value: payload.OtherFiles})
		

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