package main

import (
	"log"
	"net/http"

	firebase "github.com/IEEECS-VIT/hackbattle25-backend/config"
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
	firebase.InitFirebase()

	//Registering routes
	routes.AuthRoutes(router, firebase.App)
	routes.RegisterTeamRoutes(router)

	log.Println("Server is running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", router))
}
