package main

import (
	"context"
	"log"
	"net/http"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
	"github.com/gorilla/mux"
)

func main() {
	//Firebase initialization
	ctx := context.Background()

	opt := option.WithCredentialsFile("serviceAccountKey.json")

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing firebase app: %v\n", err)
	}
	log.Println("Firebase app initialized successfully.")

	_, err = app.Auth(ctx)
	if err != nil {
		log.Fatalf("Failed to verify connection with Firebase: %v", err)
	}
	log.Println("Successfully verified connection to Firebase services!")

	//Server setup
	r := mux.NewRouter()

	// routes.AuthRoutes(r, app)
	log.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
