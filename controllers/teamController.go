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

func getUIDFromContext(r *http.Request) (string, bool) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		return "", false
	}
	return token.UID, true
}

func verifyTeamLeader(ctx context.Context, r *http.Request) (string, error) {
	// 1. Get both email and UID from the token via the request context
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		return "", &httpError{"Invalid token: missing email", http.StatusUnauthorized}
	}
	uid, ok := getUIDFromContext(r)
	if !ok {
		return "", &httpError{"Invalid token: missing UID", http.StatusUnauthorized}
	}

	// 2. Look up the user document using their EMAIL as the document ID
	userDoc, err := config.FirestoreClient.Collection("users").Doc(userEmail).Get(ctx)
	if err != nil {
		return "", &httpError{"User profile not found", http.StatusNotFound}
	}

	// 3. Check the 'IsLead' field in the user's document
	isLead, err := userDoc.DataAt("IsLead")
	if err != nil || !isLead.(bool) {
		return "", &httpError{"User is not a team leader", http.StatusForbidden}
	}

	// 4. Get the TeamID from the user's document
	teamID, err := userDoc.DataAt("TeamID")
	if err != nil {
		return "", &httpError{"Team ID not found for leader", http.StatusInternalServerError}
	}
	teamIDStr := teamID.(string)

	// 5. Final check for data consistency
	teamDoc, err := config.FirestoreClient.Collection("teams").Doc(teamIDStr).Get(ctx)
	if err != nil {
		return "", &httpError{"Team data is inconsistent or missing", http.StatusInternalServerError}
	}
	leaderUID, _ := teamDoc.DataAt("leaderId")
	if leaderUID != uid {
		return "", &httpError{"Leadership mismatch in team data", http.StatusForbidden}
	}

	return teamIDStr, nil
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
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}
	uid, ok := getUIDFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing UID", http.StatusUnauthorized)
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
			"leaderId":  uid,
			"members":   []string{uid},
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
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}
	uid, ok := getUIDFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing UID", http.StatusUnauthorized)
		return
	}

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamCode == "" {
		http.Error(w, "Invalid team code provided", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
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
			{Path: "members", Value: firestore.ArrayUnion(uid)},
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
    userEmail, ok := getUserEmailFromContext(r)
    if !ok {
        http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
        return
    }
    uid, ok := getUIDFromContext(r)
    if !ok {
        http.Error(w, "Invalid token: missing UID", http.StatusUnauthorized)
        return
    }
    ctx := context.Background()
    userRef := config.FirestoreClient.Collection("users").Doc(userEmail)

    err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
        userDoc, err := tx.Get(userRef)
        if err != nil {
            return status.Errorf(codes.NotFound, "User profile not found")
        }

        teamIDValue, err := userDoc.DataAt("TeamID")
        if err != nil || teamIDValue == nil {
            return status.Errorf(codes.FailedPrecondition, "User is not in a team")
        }
        teamID, ok := teamIDValue.(string)
        if !ok || teamID == "" {
            return status.Errorf(codes.Internal, "TeamID has an invalid format")
        }

		//check if its leader
        isLeadData, err := userDoc.DataAt("IsLead")
        if err == nil {
            if isLead, ok := isLeadData.(bool); ok && isLead {
                 return status.Errorf(codes.FailedPrecondition, "Leaders cannot leave a team. You must delete the team or transfer leadership.")
            }
        }

        teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
        teamUpdates := []firestore.Update{
            {Path: "members", Value: firestore.ArrayRemove(uid)},
            {Path: "emails", Value: firestore.ArrayRemove(userEmail)},
        }
        if err := tx.Update(teamRef, teamUpdates); err != nil {
             if status.Code(err) == codes.NotFound {
                 return status.Errorf(codes.NotFound, "Team data not found, record is inconsistent")
            }
            return err
        }
        
        userUpdates := []firestore.Update{
            {Path: "TeamID", Value: firestore.Delete}, 
            {Path: "IsLead", Value: firestore.Delete}, 
        }
        return tx.Update(userRef, userUpdates)
    })

    if err != nil {
        handleFirestoreError(w, err)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Successfully left team"})
}

func GetTeam(w http.ResponseWriter, r *http.Request) {
	userEmail, ok := getUserEmailFromContext(r)
	if !ok {
		http.Error(w, "Invalid token: missing email", http.StatusUnauthorized)
		return
	}
	ctx := context.Background()

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userEmail).Get(ctx)
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
	memberEmails, err := teamDoc.DataAt("emails")
	if err == nil {
		usersCollection := config.FirestoreClient.Collection("users")
		for _, memberEmail := range memberEmails.([]interface{}) {
			emailStr := memberEmail.(string)
			memberDoc, err := usersCollection.Doc(emailStr).Get(ctx)
			if err == nil {
				var name, uid string
				if n, ok := memberDoc.Data()["Name"].(string); ok {
					name = n
				}
				if u, ok := memberDoc.Data()["UID"].(string); ok {
					uid = u
				}
				memberDetailsList = append(memberDetailsList, MemberDetails{
					UID:   uid,
					Name:  name,
					Email: emailStr,
				})
			}
		}
	}

	leaderID, _ := teamDoc.DataAt("leaderId")

	response := map[string]interface{}{
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
	ctx := context.Background()

	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Name == "" {
		http.Error(w, "Invalid team name provided", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, r)
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
	ctx := context.Background()

	var payload struct {
		MemberUID string `json:"memberUid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.MemberUID == "" {
		http.Error(w, "Invalid member UID provided", http.StatusBadRequest)
		return
	}

	teamID, err := verifyTeamLeader(ctx, r)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	leaderUID, _ := getUIDFromContext(r)
	if leaderUID == payload.MemberUID {
		http.Error(w, "Leader cannot remove themselves from the team", http.StatusForbidden)
		return
	}


	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// 1. Get the team document first to verify the member exists.
		teamDoc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}

		members, err := teamDoc.DataAt("members")
		if err != nil {
			return &httpError{"Could not read team members", http.StatusInternalServerError}
		}

		// 2. Check if the member to be removed is actually in the team.
		isMember := false
		for _, member := range members.([]interface{}) {
			if member.(string) == payload.MemberUID {
				isMember = true
				break
			}
		}
		if !isMember {
			return &httpError{"User is not a member of this team", http.StatusNotFound}
		}

		// 3. Find the user document for the member to get their email.
		usersCollection := config.FirestoreClient.Collection("users")
		query := usersCollection.Where("UID", "==", payload.MemberUID).Limit(1)
		iter := tx.Documents(query)
		memberUserDoc, err := iter.Next()
		if err != nil {
			return &httpError{"Member user profile not found", http.StatusNotFound}
		}
		
		memberEmail := memberUserDoc.Ref.ID
		memberUserRef := memberUserDoc.Ref

		// 4. Remove the member's UID and email from the team document.
		teamUpdates := []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(payload.MemberUID)},
			{Path: "emails", Value: firestore.ArrayRemove(memberEmail)},
		}
		if err := tx.Update(teamRef, teamUpdates); err != nil {
			return err
		}

		// 5. Clear the team details from the member's user document.
		return tx.Update(memberUserRef, []firestore.Update{
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

//define the structure for project submission.
type SubmissionPayload struct {
	ProblemStmt *string  `json:"problem_stmt"`
	GithubLink  *string  `json:"github_link"`
	FigmaLink   *string  `json:"figma_link"`
	OtherFiles  []string `json:"other_files"`
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
