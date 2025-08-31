package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/iterator"
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

		// 3. Check if the user's email is in the 'registrations' collection in Firestore
		userEmail := token.Claims["email"].(string)
		query := firestoreClient.Collection("registrations").Where("email", "==", userEmail).Limit(1)
		iter := query.Documents(context.Background())
		_, err = iter.Next()

		if err == iterator.Done {
			// No documents found, user is not registered
			log.Printf("Sign-in failed: email '%s' not found in registrations.", userEmail)
			http.Error(w, "User has not registered for the event.", http.StatusForbidden)
			return
		}
		if err != nil {
			log.Printf("Error querying Firestore: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// 4. User is registered, sign-in is successful
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Sign-in successful",
			"uid":     token.UID,
			"email":   userEmail,
		})
		log.Printf("Successfully signed in user: %s (Email: %s)", token.UID, userEmail)
	}
}
