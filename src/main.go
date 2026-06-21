package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"task-manager/src/config"
	"task-manager/src/db"
	"task-manager/src/handlers"
	"task-manager/src/middleware"
	"task-manager/src/service"
)

func main() {
	// 1. Загружаем конфиг
	cfg := config.LoadConfig()

	// 2. Инициализируем БД и Redis
	clients, err := db.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer clients.DB.Close()
	defer clients.Redis.Close()

	// 3. Инициализируем сервисы
	authService := service.NewAuthService(clients.DB, cfg)
	teamService := service.NewTeamService(clients.DB, clients.Redis)
	taskService := service.NewTaskService(clients.DB, clients.Redis, teamService)
	commentService := service.NewCommentService(clients.DB, taskService)

	// 4. Инициализируем хендлеры
	authHandler := handlers.NewAuthHandler(authService)
	teamHandler := handlers.NewTeamHandler(teamService)
	taskHandler := handlers.NewTaskHandler(taskService)
	commentHandler := handlers.NewCommentHandler(commentService)

	// 5. Создаем роутер Gin
	r := gin.Default()

	// Группа API v1
	v1 := r.Group("/api/v1")
	{
		// Публичные эндпоинты (Регистрация и Логин)
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)

		// Защищенные эндпоинты (Требуют JWT токен)
		protected := v1.Group("/")
		protected.Use(middleware.AuthMiddleware(cfg))
		{
			// Управление командами
			protected.POST("/teams", teamHandler.CreateTeam)
			protected.GET("/teams", teamHandler.GetTeams)
			protected.POST("/teams/:id/invite", teamHandler.InviteMember)
			protected.GET("/teams/:id/analytics", taskHandler.GetTeamAnalytics)

			// Управление задачами в командах
			protected.POST("/teams/:id/tasks", taskHandler.CreateTask)
			protected.GET("/teams/:id/tasks", taskHandler.GetTeamTasks)

			// Управление конкретной задачей
			protected.GET("/tasks/:id", taskHandler.GetTaskByID)
			protected.PATCH("/tasks/:id", taskHandler.UpdateTask)
			protected.GET("/tasks/:id/history", taskHandler.GetTaskHistory)

			// Комментарии к задачам
			protected.POST("/tasks/:id/comments", commentHandler.CreateComment)
			protected.GET("/tasks/:id/comments", commentHandler.GetTaskComments)
		}
	}

	// Запуск сервера на порту из конфигурации
	log.Printf("Server is running on port %s...", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
