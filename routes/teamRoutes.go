package routes

import (
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
	"github.com/gorilla/mux"
)

func RegisterTeamRoutes(r *mux.Router) {
	r.Handle("/teams/create", middleware.AuthMiddleware(http.HandlerFunc(controllers.CreateTeam))).Methods("POST")
	r.Handle("/teams/join", middleware.AuthMiddleware(http.HandlerFunc(controllers.JoinTeam))).Methods("POST")
	r.Handle("/teams/get", middleware.AuthMiddleware(http.HandlerFunc(controllers.GetTeam))).Methods("GET")
	r.Handle("/teams/remove-member", middleware.AuthMiddleware(http.HandlerFunc(controllers.RemoveMember))).Methods("POST")
	r.Handle("/teams/delete", middleware.AuthMiddleware(http.HandlerFunc(controllers.DeleteTeam))).Methods("DELETE")
	r.Handle("/teams/leave-team", middleware.AuthMiddleware(http.HandlerFunc(controllers.LeaveOrDeleteTeam))).Methods("DELETE")
	r.Handle("/teams/project/submit", middleware.AuthMiddleware(http.HandlerFunc(controllers.SubmitProject))).Methods("POST")
	r.Handle("/teams/project/update", middleware.AuthMiddleware(http.HandlerFunc(controllers.UpdateProject))).Methods("PUT")
}
