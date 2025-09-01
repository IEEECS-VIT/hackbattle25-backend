package main

import (
	"log"
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/routes"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Router is working!"))
	}).Methods("GET")

	config.InitFirebase()

	routes.RegisterAuthRoutes(router, config.AuthClient, config.FirestoreClient)
	routes.RegisterTeamRoutes(router)
	routes.RegisterSubmitRoutes(router)

	log.Println("Server is running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", router))
}
