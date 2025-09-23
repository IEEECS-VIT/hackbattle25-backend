package controllers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
	"github.com/IEEECS-VIT/hackbattle25-backend/models"
	"github.com/gorilla/mux"
	"github.com/xuri/excelize/v2"
	"google.golang.org/api/iterator"
)

type EmailData struct {
	Email string `json:"email" firestore:"email"`
}

func ImportEmailsFromExcel(w http.ResponseWriter, r *http.Request) {

	r.ParseMultipartForm(10 << 20) // 10 MB max file size
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	f, err := excelize.OpenReader(file)
	if err != nil {
		http.Error(w, "Failed to open Excel file", http.StatusInternalServerError)
		return
	}

	rows, err := f.GetRows(f.GetSheetList()[0])
	if err != nil {
		http.Error(w, "Failed to get rows from Excel sheet", http.StatusInternalServerError)
		return
	}

	if len(rows) <= 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Excel file is empty or contains only a header row. No emails were imported."))
		return
	}

	for _, row := range rows[1:] {
		if len(row) > 0 {
			email := row[0]
			if email != "" {
				docRef := config.FirestoreClient.Collection("users").Doc(email)
				_, err := docRef.Set(context.Background(), map[string]interface{}{"email": email})
				if err != nil {
					fmt.Printf("Error adding email %s to firestore: %v\n", email, err)
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Emails uploaded successfully!"))
}

func ExportTeamsToExcel(w http.ResponseWriter, r *http.Request) {
	f := excelize.NewFile()
	sheetName := "Teams"
	f.SetSheetName("Sheet1", sheetName)

	// Set headers for all fields
	headers := []string{
		"Team Name", "Team Code", "Type", "Leader ID", "Emails", "Names",
		"Problem Statement", "GitHub Link", "Figma Link", "Other Files",
		"Submitted At", "Updated At", "Created At",
	}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	ctx := context.Background()
	docs := config.FirestoreClient.Collection("teams").Documents(ctx)
	defer docs.Stop()

	rowIndex := 2
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Failed to iterate team documents: %v", err)
			http.Error(w, "Failed to retrieve team data", http.StatusInternalServerError)
			return
		}

		var team models.Team
		if err := doc.DataTo(&team); err != nil {
			log.Printf("Failed to parse team data for doc %s: %v", doc.Ref.ID, err)
			continue
		}

		var emailsSlice []string
		var namesSlice []string

		if membersArray, ok := doc.Data()["members"].([]interface{}); ok {
			for _, item := range membersArray {
				if memberMap, ok := item.(map[string]interface{}); ok {
					if email, ok := memberMap["email"].(string); ok {
						emailsSlice = append(emailsSlice, email)
					}
					if name, ok := memberMap["name"].(string); ok {
						namesSlice = append(namesSlice, name)
					}
				}
			}
		}

		emails := strings.Join(emailsSlice, ", ")
		names := strings.Join(namesSlice, ", ")

		problemStmt := ""
		if team.ProblemStmt != nil {
			problemStmt = *team.ProblemStmt
		}
		githubLink := ""
		if team.GithubLink != nil {
			githubLink = *team.GithubLink
		}
		figmaLink := ""
		if team.FigmaLink != nil {
			figmaLink = *team.FigmaLink
		}
		otherFiles := ""
		if team.OtherFiles != nil {
			otherFiles = *team.OtherFiles
		}
		submittedAt := ""
		if team.SubmittedAt != nil {
			submittedAt = team.SubmittedAt.Format("2006-01-02 15:04:05")
		}
		updatedAt := ""
		if team.UpdatedAt != nil {
			updatedAt = team.UpdatedAt.Format("2006-01-02 15:04:05")
		}
		createdAt := team.CreatedAt.Format("2006-01-02 15:04:05")

		teamType := "Internal"
		if strings.Split(team.LeaderID, "@")[1] != "vitstudent.ac.in" {
			teamType = "External"
		}

		rowData := []interface{}{
			team.Name,
			team.Code,
			teamType,
			team.LeaderID,
			emails,
			names,
			problemStmt,
			githubLink,
			figmaLink,
			otherFiles,
			submittedAt,
			updatedAt,
			createdAt,
		}

		for i, data := range rowData {
			cell, _ := excelize.CoordinatesToCellName(i+1, rowIndex)
			f.SetCellValue(sheetName, cell, data)
		}
		rowIndex++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		log.Printf("Failed to write excel file to buffer: %v", err)
		http.Error(w, "Failed to write Excel file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=teams_export.xlsx")
	w.Write(buf.Bytes())
}

func RemoveTeams(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	teamID, ok := vars["teamID"]
	if !ok {
		http.Error(w, "Team ID is missing in request", http.StatusBadRequest)
		return
	}

	docRef := config.FirestoreClient.Collection("teams").Doc(teamID)

	_, err := docRef.Delete(context.Background())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete team: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Team with ID '%s' successfully deleted.", teamID)))
}
