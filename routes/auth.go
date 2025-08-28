package routes

import (
	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/gorilla/mux"
)

func AuthRoutes(r *mux.Router) {
	r.HandleFunc("/signup", controllers.Signup).Methods("POST")
	r.HandleFunc("/login", controllers.Login).Methods("POST")
}
