package main

import (
	"log"
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/routes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()

	// Healthcheck Route
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Router is working!"))
	}).Methods("GET")

	// Initialize Firebase & Firestore
	config.InitFirebase()

	// Register Application Routes
	routes.RegisterAuthRoutes(router, config.AuthClient, config.FirestoreClient)
	routes.RegisterTeamRoutes(router)

	// Configure CORS Middleware
	allowedOrigins := handlers.AllowedOrigins([]string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://localhost:3002",
		"http://127.0.0.1:5500", 
		"https://hackbattle.ieeecsvit.com",
		"https://elegant-hotteok-e0afec.netlify.app",
		"https://hackbattle25.netlify.app",
		"https://hackbattle-26.vercel.app",
		"https://hackbattle26.ieeecsvit.com",
		"https://hackbattle-26-frontend.vercel.app",

	})
	allowedMethods := handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	allowedHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})
	allowCredentials := handlers.AllowCredentials()

	corsHandler := handlers.CORS(allowedOrigins, allowedMethods, allowedHeaders, allowCredentials)(router)

	log.Println("Server running on http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", corsHandler))
}