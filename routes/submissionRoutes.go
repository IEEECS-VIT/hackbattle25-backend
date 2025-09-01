package routes

import (
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
	"github.com/gorilla/mux"
)

func RegisterSubmitRoutes(r *mux.Router) {
	r.Handle("/tasks/submit", middleware.AuthMiddleware(http.HandlerFunc(controllers.SubmitTasks))).Methods("POST")
	r.Handle("/tasks/update", middleware.AuthMiddleware(http.HandlerFunc(controllers.UpdateSubmission))).Methods("PUT")
}
