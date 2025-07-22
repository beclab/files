package goseahub

//import (
//	"context"
//	"errors"
//	"files/pkg/goseahub/models"
//	"k8s.io/klog/v2"
//	"log"
//	"net/http"
//)
//
//func AuthMiddleware(next http.Handler) http.Handler {
//	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		userHeader := r.Header.Get("X-Bfl-User")
//		if userHeader == "" {
//			log.Println("Invalid header format: missing X-Bfl-User")
//			http.Error(w, "Invalid header format", http.StatusBadRequest)
//			return
//		}
//
//		username := userHeader + "@seafile.com"
//		log.Printf("Processing username: %s", username)
//
//		allUsers, err := ListAllUsers()
//		if err != nil {
//			log.Printf("Error listing users: %v", err)
//			http.Error(w, "Internal server error", http.StatusInternalServerError)
//			return
//		}
//
//		existedUser, exists := allUsers[username]
//		if !exists || existedUser["email"] == "" {
//			log.Printf("User not found: %s", username)
//			http.Error(w, "User not found", http.StatusUnauthorized)
//			return
//		}
//
//		virtualEmail := existedUser["email"]
//		log.Printf("Found virtual email: %s", virtualEmail)
//
//		user, err := models.GlobalProfileManager.GetProfileByUser(virtualEmail.(string))
//		if err != nil {
//			if errors.Is(err, models.ErrProfileNotFound) {
//				log.Printf("User not found with email: %s", virtualEmail)
//				http.Error(w, "User not found", http.StatusUnauthorized)
//				return
//			}
//			klog.Errorf("Database error: %v", err)
//			http.Error(w, "Internal server error", http.StatusInternalServerError)
//			return
//		}
//
//		ctx := context.WithValue(r.Context(), "user", user)
//		next.ServeHTTP(w, r.WithContext(ctx))
//	})
//}
