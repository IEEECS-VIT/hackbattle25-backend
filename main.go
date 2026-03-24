package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/routes"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	return origins
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Router is working!"))
	}).Methods("GET")

	config.InitFirebase()

	routes.RegisterAuthRoutes(router, config.AuthClient, config.FirestoreClient)
	routes.RegisterTeamRoutes(router)

	defaultAllowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://localhost:3002",
		"https://hackbattle.ieeecsvit.com",
		"https://elegant-hotteok-e0afec.netlify.app",
		"https://hackbattle25.netlify.app",
		"https://hackbattle-25.vercel.app",
		"https://hackbattle25.ieeecsvit.com",
		"https://hackbattle.ieeecsvit.com",
		"https://hackbattle-25-wovl.vercel.app",
	}

	allowedOriginsList := defaultAllowedOrigins
	if rawAllowedOrigins := os.Getenv("ALLOWED_ORIGINS"); rawAllowedOrigins != "" {
		if parsed := parseAllowedOrigins(rawAllowedOrigins); len(parsed) > 0 {
			allowedOriginsList = parsed
		} else {
			log.Println("ALLOWED_ORIGINS is set but empty after parsing; falling back to defaults")
		}
	}

	log.Printf("CORS allowed origins: %v\n", allowedOriginsList)
	allowedOrigins := handlers.AllowedOrigins(allowedOriginsList)
	allowedMethods := handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	allowedHeaders := handlers.AllowedHeaders([]string{"Content-Type", "Authorization"})
	allowCredentials := handlers.AllowCredentials()

	log.Println("Server is running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", handlers.CORS(allowedOrigins, allowedMethods, allowedHeaders, allowCredentials)(router)))
}
