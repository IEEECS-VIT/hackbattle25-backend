package firebase

import (
	"context"
	"log"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

var App *firebase.App

func InitFirebase() {
	ctx := context.Background()
	opt := option.WithCredentialsFile("serviceAccountKey.json")

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	log.Println("Firebase app initialized using environment variable.", app)

	App = app

	_, err = app.Auth(ctx)
	if err != nil {
		log.Fatalf("Failed to verify connection with Firebase: %v", err)
	}
	log.Println("Successfully verified connection to Firebase services!")
}
