package routes

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/gorilla/mux"
)

func AuthRoutes(r *mux.Router, app *firebase.App) {

	authClient, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	r.HandleFunc("/signup", controllers.Register(authClient)).Methods("POST")
}