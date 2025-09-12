package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/IEEECS-VIT/hackbattle25-backend/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GoogleSignInPayload struct {
	IDToken string `json:"idToken"`
}

func SignIn(authClient *auth.Client, firestoreClient *firestore.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Decode the incoming JSON payload to get the Google ID token
		var payload GoogleSignInPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		if payload.IDToken == "" {
			http.Error(w, "Missing ID token", http.StatusBadRequest)
			return
		}

		// 2. Verify the Google ID token using Firebase Auth
		token, err := authClient.VerifyIDToken(context.Background(), payload.IDToken)
		if err != nil {
			log.Printf("error verifying ID token: %v\n", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// 3. Check if the user's email exists in the 'users' collection in Firestore
		userEmail := strings.ToLower(token.Claims["email"].(string)) // normalize to lowercase
		ctx := context.Background()

		userRef := firestoreClient.Collection("users").Doc(userEmail)
		_, err = userRef.Get(ctx)

		if status.Code(err) == codes.NotFound {
			// No document found, user is not registered
			log.Printf("Sign-in failed: email '%s' not found in registrations.", userEmail)
			http.Error(w, "User has not registered for the event.", http.StatusForbidden)
			return
		}
		if err != nil {
			log.Printf("Error checking Firestore: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 4. User is registered. Check if a user profile document exists, if not, create it.
		profileRef := firestoreClient.Collection("users").Doc(userEmail) // same email ID
		_, err = profileRef.Get(ctx)

		if status.Code(err) == codes.NotFound {
			log.Printf("First sign-in for user %s. Creating user document.", userEmail)
			newUser := models.User{
				Name:      token.Claims["name"].(string),
				Email:     userEmail,
				TeamID:    nil, // User has no team initially
				IsLead:    false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if _, createErr := profileRef.Set(ctx, newUser); createErr != nil {
				log.Printf("Failed to create user document for email %s: %v", userEmail, createErr)
				http.Error(w, "Failed to create user profile", http.StatusInternalServerError)
				return
			}
		} else if err != nil {
			// Any other error during the Get operation is a server issue.
			log.Printf("Error checking for user document %s: %v", userEmail, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 5. User is registered and has a user document, sign-in is successful
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Sign-in successful",
			"email":   userEmail,
		})
		log.Printf("Successfully signed in user: %s", userEmail)

	}
}
