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
)

type TeamPayload struct {
	Name string `json:"name"`
	Code string `json:"code"`
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

	teams := config.FirestoreClient.Collection("teams")
	q := teams.Where("Name", "==", team.Name).Limit(1)
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

	doc := map[string]interface{}{
		"Name":    team.Name,
		"Code":    teamCode,
		"members": []string{userID},
		"emails":  []string{userEmail},
	}

	_, _, err = teams.Add(ctx, doc)
	if err != nil {
		http.Error(w, "Failed to create team", http.StatusInternalServerError)
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
	teams := config.FirestoreClient.Collection("teams")

	q := teams.Where("Code", "==", req.TeamCode).Limit(1)
	docs, err := q.Documents(ctx).GetAll()
	if err != nil || len(docs) == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	teamRef := docs[0].Ref
	teamSnap, err := teamRef.Get(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch team data", http.StatusInternalServerError)
		return
	}

	members, _ := teamSnap.Data()["members"].([]interface{})
	for _, m := range members {
		if m.(string) == userID {
			http.Error(w, "User already in team", http.StatusConflict)
			return
		}
	}

	_, err = teamRef.Update(ctx, []firestore.Update{
		{Path: "members", Value: firestore.ArrayUnion(userID)},
	})
	if err != nil {
		http.Error(w, "Failed to join team", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User joined team successfully",
		"team_id": teamRef.ID,
	})
}

func GetTeam(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(middleware.UserKey).(*auth.Token)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := token.UID

	var req struct {
		TeamCode string `json:"team_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamCode == "" {
		http.Error(w, "Missing or invalid team_code in body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	teams := config.FirestoreClient.Collection("teams")

	q := teams.Where("Code", "==", req.TeamCode).Limit(1)
	docs, err := q.Documents(ctx).GetAll()
	if err != nil || len(docs) == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	teamData := docs[0].Data()
	members := []string{}
	if ms, ok := teamData["members"].([]interface{}); ok {
		for _, m := range ms {
			if s, ok := m.(string); ok {
				members = append(members, s)
			}
		}
	}

	isMember := false
	for _, m := range members {
		if m == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		http.Error(w, "Forbidden: you are not a member of this team", http.StatusForbidden)
		return
	}

	name, _ := teamData["Name"].(string)
	code, _ := teamData["Code"].(string)

	resp := map[string]interface{}{
		"name":    name,
		"code":    code,
		"members": members,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
