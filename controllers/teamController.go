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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TeamPayload struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

const maxTeamSize = 5

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
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := token.UID
	userEmail := strings.ToLower(token.Claims["email"].(string))

	var team TeamPayload
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
	teamsCollection := config.FirestoreClient.Collection("teams")

	q := teamsCollection.Where("Name", "==", team.Name).Limit(1)
	docs, err := q.Documents(ctx).GetAll()
	if err != nil {
		http.Error(w, "Failed to check team name", http.StatusInternalServerError)
		return
	}
	if len(docs) > 0 {
		http.Error(w, "Team name already exists", http.StatusConflict)
		return
	}

	teamCode := generateTeamCode()

	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}
		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			return status.Errorf(codes.AlreadyExists, "User is already in a team")
		}

		newTeamRef := teamsCollection.NewDoc()
		err = tx.Set(newTeamRef, map[string]interface{}{
			"Name":     team.Name,
			"Code":     teamCode,
			"leaderId": userID,
			"members":  []string{userEmail},
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
		if status.Code(err) == codes.AlreadyExists {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to create team"+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Team created successfully",
		"code":    teamCode,
	})
}

func JoinTeam(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userEmail := strings.ToLower(token.Claims["email"].(string))

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userEmail)
	teamsCollection := config.FirestoreClient.Collection("teams")

	err := config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		userDoc, err := tx.Get(userRef)
		if err != nil {
			return status.Errorf(codes.NotFound, "User profile not found")
		}
		if teamID, _ := userDoc.DataAt("TeamID"); teamID != nil {
			return status.Errorf(codes.AlreadyExists, "User is already in a team")
		}

		q := teamsCollection.Where("Code", "==", req.TeamCode).Limit(1)
		docs, err := tx.Documents(q).GetAll()
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return status.Errorf(codes.NotFound, "Team with that code not found")
		}

		teamRef := docs[0].Ref
		teamSnap, err := tx.Get(teamRef)
		if err != nil {
			return err
		}

		members, _ := teamSnap.DataAt("members")
		if len(members.([]interface{})) >= maxTeamSize {
			return status.Errorf(codes.FailedPrecondition, "Team is already full")
		}

		if err := tx.Update(teamRef, []firestore.Update{
			{Path: "members", Value: firestore.ArrayUnion(userEmail)},
		}); err != nil {
			return err
		}
		return tx.Update(userRef, []firestore.Update{{Path: "TeamID", Value: teamRef.ID}})
	})

	if err != nil {
		st := status.Convert(err)
		switch st.Code() {
		case codes.AlreadyExists, codes.NotFound, codes.FailedPrecondition:
			http.Error(w, st.Message(), http.StatusConflict)
		default:
			http.Error(w, "Failed to join team", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User joined team successfully",
	})
}

func LeaveTeam(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userEmail := strings.ToLower(token.Claims["email"].(string))
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
		teamID := teamIDValue.(string)

		isLead, _ := userDoc.DataAt("IsLead")
		if isLead.(bool) {
			return status.Errorf(codes.FailedPrecondition, "Leaders cannot leave a team. You must delete the team or transfer leadership.")
		}

		teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
		if err := tx.Update(teamRef, []firestore.Update{
			{Path: "members", Value: firestore.ArrayRemove(userEmail)},
		}); err != nil {
			return err
		}
		return tx.Update(userRef, []firestore.Update{
			{Path: "TeamID", Value: nil},
			{Path: "IsLead", Value: false},
		})
	})

	if err != nil {
		st := status.Convert(err)
		if st.Code() == codes.FailedPrecondition {
			http.Error(w, st.Message(), http.StatusForbidden)
		} else {
			http.Error(w, "Failed to leave team", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Successfully left team"})
}

func GetTeam(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userEmail := strings.ToLower(token.Claims["email"].(string))
	ctx := context.Background()

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userEmail).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, "User profile not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve user profile", http.StatusInternalServerError)
		}
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
		http.Error(w, "Failed to retrieve team details", http.StatusInternalServerError)
		return
	}

	var teamData struct {
		Name     string   `firestore:"Name"`
		Code     string   `firestore:"Code"`
		LeaderID string   `firestore:"leaderId"`
		Members  []string `firestore:"members"`
	}
	if err := teamDoc.DataTo(&teamData); err != nil {
		http.Error(w, "Failed to parse team data", http.StatusInternalServerError)
		return
	}

	type MemberDetails struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	memberDetailsList := make([]MemberDetails, 0, len(teamData.Members))
	for _, memberEmail := range teamData.Members {
		memberDoc, err := config.FirestoreClient.Collection("users").Doc(memberEmail).Get(ctx)
		if err == nil {
			memberDetailsList = append(memberDetailsList, MemberDetails{
				Email: memberEmail,
				Name:  memberDoc.Data()["Name"].(string),
			})
		}
	}

	response := map[string]interface{}{
		"id":       teamDoc.Ref.ID,
		"name":     teamData.Name,
		"code":     teamData.Code,
		"leaderId": teamData.LeaderID,
		"members":  memberDetailsList,
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

	//only leader can change
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

		if err := tx.Update(teamRef, []firestore.Update{{Path: "members", Value: firestore.ArrayRemove(payload.MemberID)}}); err != nil {
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

	// 1. Verify the user is a team leader and get their teamID.
	teamID, err := verifyTeamLeader(ctx, leaderID)
	if err != nil {
		handleFirestoreError(w, err)
		return
	}

	teamRef := config.FirestoreClient.Collection("teams").Doc(teamID)
	leaderRef := config.FirestoreClient.Collection("users").Doc(leaderID)

	// 2. Use a transaction to ensure the entire operation is atomic.
	err = config.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		teamDoc, err := tx.Get(teamRef)
		if err != nil {
			return &httpError{"Team not found", http.StatusNotFound}
		}

		// 3. Check if there are other members in the team.
		members, err := teamDoc.DataAt("members")
		if err != nil {
			return err // Should not happen if data is consistent.
		}

		if len(members.([]interface{})) > 1 {
			return &httpError{"You must remove all other members before deleting the team", http.StatusForbidden}
		}

		// 4. If only the leader is left, delete the team document.
		if err := tx.Delete(teamRef); err != nil {
			return err
		}

		// 5. Update the leader's user document to remove them from the team.
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
