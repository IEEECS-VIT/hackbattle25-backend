package routes

import (
	"net/http"

	"github.com/IEEECS-VIT/hackbattle25-backend/controllers"
	"github.com/IEEECS-VIT/hackbattle25-backend/middleware"
	"github.com/gorilla/mux"
)

func RegisterAdminRoutes(r *mux.Router) {
	r.Handle("/admin/import-emails", middleware.AdminAuthMiddleware(http.HandlerFunc(controllers.ImportEmailsFromExcel))).Methods("POST")
	r.Handle("/admin/export-teams", middleware.AdminAuthMiddleware(http.HandlerFunc(controllers.ExportTeamsToExcel))).Methods("GET")
	r.Handle("/admin/remove-teams/{teamID}", middleware.AdminAuthMiddleware(http.HandlerFunc(controllers.RemoveTeams))).Methods("DELETE")
}
