package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/wersvet/chat_1/proto/auth"
	userpb "github.com/wersvet/user-service/proto/user"

	"chat-service/internal/db"
	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/handlers"
	"chat-service/internal/middleware"
	"chat-service/internal/repositories"
	"chat-service/internal/ws"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	authAddr := getEnv("AUTH_GRPC_ADDR", "localhost:8084")
	userAddr := getEnv("USER_GRPC_ADDR", "localhost:8085")

	authConn, err := grpc.Dial(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to auth grpc: %v", err)
	}
	defer authConn.Close()

	userConn, err := grpc.Dial(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to user grpc: %v", err)
	}
	defer userConn.Close()

	authClient := grpcclient.NewAuthClient(authpb.NewAuthServiceClient(authConn))
	userClient := grpcclient.NewUserClient(userpb.NewUserInternalClient(userConn))

	chatRepo := repositories.NewChatRepo(database)
	messageRepo := repositories.NewMessageRepo(database)

	hub := ws.NewHub()

	chatHandler := handlers.NewChatHandler(chatRepo, messageRepo, userClient, hub)

	chatWS := ws.NewChatWebSocketHandler(hub, chatRepo, authClient)

	router := gin.Default()

	// middlewares
	router.Use(gin.Recovery())

	authMiddleware := middleware.AuthMiddleware(authClient)

	router.GET("/chats", authMiddleware, chatHandler.ListChats)
	router.POST("/chats/start", authMiddleware, chatHandler.StartChat)
	router.GET("/chats/:chat_id/messages", authMiddleware, chatHandler.GetChatMessages)
	router.POST("/chats/:chat_id/messages", authMiddleware, chatHandler.PostChatMessage)
	router.DELETE("/chats/:chat_id/messages/:message_id/me", authMiddleware, chatHandler.DeleteMessageForMe)
	router.DELETE("/chats/:chat_id/messages/:message_id/all", authMiddleware, chatHandler.DeleteMessageForAll)
	router.DELETE("/chats/:chat_id/me", authMiddleware, chatHandler.DeleteChatForMe)

	router.GET("/ws/chats/:chat_id", chatWS.Handle)

	port := getEnv("PORT", "8083")
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
