package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "chat-service/pb/auth"
	userpb "chat-service/pb/user"

	"chat-service/internal/db"
	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/handlers"
	"chat-service/internal/middleware"
	"chat-service/internal/observability"
	"chat-service/internal/repositories"
	"chat-service/internal/ws"
)

func main() {
	shutdown, err := observability.InitTracerProvider(context.Background(), "chat-service")
	if err != nil {
		log.Fatalf("failed to initialize tracing: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown tracing: %v", err)
		}
	}()

	database, err := db.Connect()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	authAddr := getEnv("AUTH_GRPC_ADDR", "localhost:8084")
	userAddr := getEnv("USER_GRPC_ADDR", "localhost:8085")

	authConn, err := grpc.Dial(
		authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	)
	if err != nil {
		log.Fatalf("failed to connect to auth grpc: %v", err)
	}
	defer authConn.Close()

	userConn, err := grpc.Dial(
		userAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	)
	if err != nil {
		log.Fatalf("failed to connect to user grpc: %v", err)
	}
	defer userConn.Close()

	authClient := grpcclient.NewAuthClient(authpb.NewAuthServiceClient(authConn))
	userClient := grpcclient.NewUserClient(userpb.NewUserInternalClient(userConn))

	chatRepo := repositories.NewChatRepo(database)
	messageRepo := repositories.NewMessageRepo(database)
	groupRepo := repositories.NewGroupRepo(database)
	groupMessageRepo := repositories.NewGroupMessageRepo(database)

	amqpURL := getEnv("AMQP_URL", "")
	amqpExchange := getEnv("AMQP_EXCHANGE", "app.logs")
	if amqpURL != "" {
		publisher, err := observability.NewAMQPPublisher(amqpURL, amqpExchange)
		if err != nil {
			log.Printf("failed to connect to amqp: %v", err)
		} else {
			observability.SetPublisher(publisher)
			defer func() {
				if err := publisher.Close(); err != nil {
					log.Printf("failed to close amqp publisher: %v", err)
				}
			}()
		}
	}

	hub := ws.NewHub()

	chatHandler := handlers.NewChatHandler(chatRepo, messageRepo, userClient, groupRepo, hub)
	groupHandler := handlers.NewGroupHandler(groupRepo, groupMessageRepo, userClient, hub)

	chatWS := ws.NewChatWebSocketHandler(hub, chatRepo, authClient)
	groupWS := ws.NewGroupWebSocketHandler(hub, groupRepo, authClient)

	router := gin.Default()

	// middlewares
	router.Use(gin.Recovery())
	router.Use(otelgin.Middleware("chat-service"))
	router.Use(observability.HTTPMetricsMiddleware())

	authMiddleware := middleware.AuthMiddleware(authClient)

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.GET("/chats", authMiddleware, chatHandler.ListChats)
	router.POST("/chats/start", authMiddleware, chatHandler.StartChat)
	router.GET("/chats/:chat_id/messages", authMiddleware, chatHandler.GetChatMessages)
	router.POST("/chats/:chat_id/messages", authMiddleware, chatHandler.PostChatMessage)
	router.DELETE("/chats/:chat_id/messages/:message_id/me", authMiddleware, chatHandler.DeleteMessageForMe)
	router.DELETE("/chats/:chat_id/messages/:message_id/all", authMiddleware, chatHandler.DeleteMessageForAll)
	router.DELETE("/chats/:chat_id/me", authMiddleware, chatHandler.DeleteChatForMe)

	router.POST("/groups", authMiddleware, groupHandler.CreateGroup)
	router.GET("/groups", authMiddleware, groupHandler.ListGroups)
	router.GET("/groups/:group_id/messages", authMiddleware, groupHandler.GetGroupMessages)
	router.POST("/groups/:group_id/messages", authMiddleware, groupHandler.PostGroupMessage)
	router.DELETE("/groups/:group_id/messages/:message_id/all", authMiddleware, groupHandler.DeleteGroupMessageForAll)

	router.GET("/ws/chats/:chat_id", chatWS.Handle)
	router.GET("/ws/groups/:group_id", groupWS.Handle)

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
