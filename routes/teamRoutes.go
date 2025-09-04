package routes

import (
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
	"github.com/gorilla/mux"
)

func RegisterTeamRoutes(r *mux.Router) {
	r.Handle("/teams/create", middleware.AuthMiddleware(http.HandlerFunc(controllers.CreateTeam))).Methods("POST")
	r.Handle("/teams/join", middleware.AuthMiddleware(http.HandlerFunc(controllers.JoinTeam))).Methods("PUT")
	r.Handle("/teams/get", middleware.AuthMiddleware(http.HandlerFunc(controllers.GetTeam))).Methods("GET")
	r.Handle("/teams/leave", middleware.AuthMiddleware(http.HandlerFunc(controllers.LeaveTeam))).Methods("DELETE")
}
