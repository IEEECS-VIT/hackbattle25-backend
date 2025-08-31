package config

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var App *firebase.App
var AuthClient *auth.Client
var FirestoreClient *firestore.Client

func InitFirebase() {
	opt := option.WithCredentialsFile("serviceAccountKey.json")
	var err error
	App, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	AuthClient, err = App.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	FirestoreClient, err = App.Firestore(context.Background())
	if err != nil {
		log.Fatalf("error getting Firestore client: %v\n", err)
	}

	log.Println("Firebase initialized successfully")
}
