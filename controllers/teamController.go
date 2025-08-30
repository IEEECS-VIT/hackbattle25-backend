package controllers

import (
	"context"
	"encoding/json"
	"net/http"

	"cloud.google.com/go/firestore"
	firebase "github.com/IEEECS-VIT/hackbattle25-backend/config"
)

type TeamPayload struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	UserName string `json:"userName"`
}

type JoinRequest struct {
	TeamName string `json:"team_name"`
	UserID   string `json:"user_id"`
}

func CreateTeam(w http.ResponseWriter, r *http.Request) {
	var team TeamPayload
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	client, err := firebase.App.Firestore(ctx)
	if err != nil {
		http.Error(w, "Failed to connect to Firestore", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	doc := map[string]interface{}{
		"Name":    team.Name,
		"Code":    team.Code,
		"members": []string{team.UserName},
	}

	_, _, err = client.Collection("teams").Add(ctx, doc)
	if err != nil {
		http.Error(w, "Failed to create team", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Team created successfully"})
}

func JoinTeam(w http.ResponseWriter, r *http.Request) {
	type JoinRequest struct {
		TeamCode string `json:"team_code"`
		UserID   string `json:"user_id"`
	}

	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	client, err := firebase.App.Firestore(ctx)
	if err != nil {
		http.Error(w, "Failed to connect to Firestore", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	teams := client.Collection("teams")
	q := teams.Where("Code", "==", req.TeamCode).Limit(1)
	docs, err := q.Documents(ctx).GetAll()
	if err != nil || len(docs) == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	teamID := docs[0].Ref.ID
	teamRef := docs[0].Ref

	teamSnap, err := teamRef.Get(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch team data", http.StatusInternalServerError)
		return
	}
	members, _ := teamSnap.Data()["Users"].([]interface{})

	for _, m := range members {
		if m.(string) == req.UserID {
			http.Error(w, "User already in team", http.StatusConflict)
			return
		}
	}

	_, err = teamRef.Update(ctx, []firestore.Update{
		{Path: "members", Value: firestore.ArrayUnion(req.UserID)},
	})
	if err != nil {
		http.Error(w, "Failed to join team", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User joined team successfully",
		"team_id": teamID,
	})
}
