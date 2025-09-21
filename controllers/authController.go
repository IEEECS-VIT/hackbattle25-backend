package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


func SignIn(authClient *auth.Client, firestoreClient *firestore.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		idToken := parts[1]

		token, err := authClient.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("error verifying ID token: %v\n", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		userEmail := strings.ToLower(token.Claims["email"].(string)) // doc ID
		ctx := context.Background()

		userRef := firestoreClient.Collection("users").Doc(userEmail)
		doc, err := userRef.Get(ctx)

		if status.Code(err) == codes.NotFound {
			_, err := userRef.Set(ctx, map[string]interface{}{
				"email":  userEmail,
				"TeamID": nil,
			})
			if err != nil {
				log.Printf("Error creating Firestore user doc: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			log.Printf("Created new user document for %s", userEmail)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]bool{
				"isInTeam": false,
			})
			return
		}

		if err != nil {
			log.Printf("Error checking Firestore: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var isInTeam bool
		TeamID, err := doc.DataAt("TeamID")
		if err == nil && TeamID != nil {
			isInTeam = true
		} else {
			isInTeam = false
		}

		log.Printf("User %s found in Firestore. TeamID: %v", userEmail, TeamID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{
			"isInTeam": isInTeam,
		})
	}
}

