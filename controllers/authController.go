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

		// 2. Verify the Google ID token using Firebase Auth
		token, err := authClient.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("error verifying ID token: %v\n", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// 3. Check if the user's email exists in the 'users' collection in Firestore
		userEmail := strings.ToLower(token.Claims["email"].(string)) // normalize to lowercase
		ctx := context.Background()

		userRef := firestoreClient.Collection("users").Doc(userEmail)
		doc, err := userRef.Get(ctx)  //get the user document

		if status.Code(err) == codes.NotFound {
			// no document found, user is not registered
			log.Printf("Sign-in failed: email '%s' not found in registrations.", userEmail)
			http.Error(w, "User has not registered for the event.", http.StatusNoContent)
			return
		}
		if err != nil {
			log.Printf("Error checking Firestore: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		var isInTeam bool = false
		TeamID , err := doc.DataAt("TeamID")

		if(err == nil && TeamID != nil){
			isInTeam = true
		} else{
			isInTeam = false
		}
		log.Printf("User %s found in Firestore. TeamID: %v", userEmail, TeamID)

		// 5. User is registered and has a user document, sign-in is successful
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{
			"isInTeam" : isInTeam,
		})
		log.Printf("Successfully signed in user: %s", userEmail)

	}
}
