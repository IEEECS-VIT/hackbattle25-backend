package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"firebase.google.com/go/v4/auth"
)

type UserPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func Register(authClient *auth.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		//decode the incoming json paykload
		var payload UserPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		//create user in firebase
		params := (&auth.UserToCreate{}).
			Email(payload.Email).
			Password(payload.Password).
			DisplayName(payload.Name)

		userRecord, err := authClient.CreateUser(context.Background(), params)
		if err != nil {
			log.Printf("error creating user: %v\n", err)
			http.Error(w, "Error creating user: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// success response with user id
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "User created successfully",
			"uid":     userRecord.UID,
		})
		log.Printf("Successfully created user: %v\n", userRecord.UID)
	}
}

