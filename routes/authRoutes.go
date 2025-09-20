package routes

import (
	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/gorilla/mux"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
)

func RegisterAuthRoutes(router *mux.Router, authClient *auth.Client, firestoreClient *firestore.Client) {
	router.HandleFunc("/signin", controllers.SignIn(authClient, firestoreClient)).Methods("GET")
}