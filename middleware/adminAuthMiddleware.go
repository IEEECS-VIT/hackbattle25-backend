package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/IEEECS-VIT/hackbattle25-backend/config"
)

var adminEmails = map[string]bool{
	"gokulsrinivasan2345@gmail.com": true,
	"aryanrameshjain@gmail.com":     true,
	"harshvardhan.p27@gmail.com":    true,
}

func AdminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		idToken := parts[1]

		token, err := config.AuthClient.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("Error verifying ID token: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		email := token.Claims["email"].(string)
		if !adminEmails[strings.ToLower(email)] {
			http.Error(w, "Forbidden: User does not have admin privileges", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, token)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
