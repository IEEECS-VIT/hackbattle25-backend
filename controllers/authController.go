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
			log.Printf("Error verifying ID token: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		emailRaw, ok := token.Claims["email"]
		if !ok {
			http.Error(w, "Token missing email claim", http.StatusUnauthorized)
			return
		}
		userEmail, ok := emailRaw.(string)
		if !ok || userEmail == "" {
			http.Error(w, "Token email claim is invalid", http.StatusUnauthorized)
			return
		}
		userEmail = strings.ToLower(userEmail)

		ctx := context.Background()
		userRef := firestoreClient.Collection("users").Doc(userEmail)
		doc, err := userRef.Get(ctx)
		if status.Code(err) == codes.NotFound {
			http.Error(w, "User has not registered for the event.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("Error checking Firestore: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		isInTeam := false
		teamIDRaw, err := doc.DataAt("TeamID")
		var teamID string
		if err == nil && teamIDRaw != nil {
			switch v := teamIDRaw.(type) {
			case string:
				teamID = v
			case *firestore.DocumentRef:
				teamID = v.ID
			default:
				log.Printf("Unexpected TeamID type: %T", v)
			}
			if teamID != "" {
				isInTeam = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{
			"isInTeam": isInTeam,
		})
	}
}


