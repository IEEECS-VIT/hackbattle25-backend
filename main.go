package main

import (
	"log"
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/routes"
	"github.com/gorilla/mux"
)

func main() {

	//Starting the server
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Router is working!"))
	}).Methods("GET")

	//Initialize Firebase
	config.InitFirebase()

	//Registering routes
    routes.RegisterAuthRoutes(router, config.AuthClient, config.FirestoreClient)

	log.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
