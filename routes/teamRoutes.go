package routes

import (
	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"

	"github.com/gorilla/mux"
)

func RegisterTeamRoutes(r *mux.Router) {
	r.HandleFunc("/teams", controllers.CreateTeam).Methods("POST")
	r.HandleFunc("/teams/join", controllers.JoinTeam).Methods("PUT")
}
