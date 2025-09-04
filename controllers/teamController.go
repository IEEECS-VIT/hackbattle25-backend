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
	userEmail := token.Claims["email"].(string)

	var team TeamPayload
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userID)
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
            "members":  []string{userID},
			"emails":  []string{userEmail},
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
            http.Error(w, "Failed to create team", http.StatusInternalServerError)
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
	userID := token.UID

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	userRef := config.FirestoreClient.Collection("users").Doc(userID)
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

        if err := tx.Update(teamRef, []firestore.Update{{Path: "members", Value: firestore.ArrayUnion(userID)}}); err != nil {
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
    userID := token.UID
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
		if err := tx.Update(teamRef, []firestore.Update{{Path: "members", Value: firestore.ArrayRemove(userID)}}); err != nil {
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
	userID := token.UID
	ctx := context.Background()

	userDoc, err := config.FirestoreClient.Collection("users").Doc(userID).Get(ctx)
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
        UID   string `json:"uid"`
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    memberDetailsList := make([]MemberDetails, 0, len(teamData.Members))
    for _, memberUID := range teamData.Members {
        memberDoc, err := config.FirestoreClient.Collection("users").Doc(memberUID).Get(ctx)
        if err == nil {
            memberDetailsList = append(memberDetailsList, MemberDetails{
                UID:   memberUID,
                Name:  memberDoc.Data()["Name"].(string),
                Email: memberDoc.Data()["Email"].(string),
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
