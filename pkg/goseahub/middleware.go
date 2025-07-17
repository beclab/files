package goseahub

import (
	"context"
	"log"
	"net/http"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求头获取用户信息
		userHeader := r.Header.Get("X-Bfl-User")
		if userHeader == "" {
			log.Println("Invalid header format: missing X-Bfl-User")
			http.Error(w, "Invalid header format", http.StatusBadRequest)
			return
		}

		// 构造用户名
		username := userHeader + "@seafile.com"
		log.Printf("Processing username: %s", username)

		// 获取所有用户列表
		allUsers, err := ListAllUsers()
		if err != nil {
			log.Printf("Error listing users: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		//// 验证用户存在性
		//existedUser, exists := allUsers[username]
		//if !exists || existedUser.Email == "" {
		//	log.Printf("User not found: %s", username)
		//	http.Error(w, "User not found", http.StatusUnauthorized)
		//	return
		//}
		//
		//// 获取虚拟邮箱
		//virtualEmail := existedUser.Email
		//log.Printf("Found virtual email: %s", virtualEmail)
		//
		//// 查询用户信息
		//user, err := GetUserByEmail(virtualEmail)
		//if err != nil {
		//	if errors.Is(err, ErrUserNotFound) {
		//		log.Printf("User not found with email: %s", virtualEmail)
		//		http.Error(w, "User not found", http.StatusUnauthorized)
		//		return
		//	}
		//	log.Printf("Database error: %v", err)
		//	http.Error(w, "Internal server error", http.StatusInternalServerError)
		//	return
		//}

		// 将用户信息存入上下文
		ctx := context.WithValue(r.Context(), "user", allUsers) //user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

//func OrgMiddleware(next http.Handler) http.Handler {
//	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		// 实现组织上下文检测逻辑
//		ctx := context.WithValue(r.Context(), "org", currentOrg)
//		next.ServeHTTP(w, r.WithContext(ctx))
//	})
//}
