package main

import (
	"log"
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/routes"
	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	routes.AuthRoutes(r)
	log.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
