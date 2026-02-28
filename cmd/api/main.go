package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"idempot/internal/config"
	handlerhttp "idempot/internal/handler/http"
	"idempot/internal/repository/migration"
	"idempot/internal/service"

	"idempot/internal/repository/postgresql"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatal("failed to load configuration:", err)
	}

	db, err := sql.Open("postgres", config.DB.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(config.DB.MaxOpenConnection)
	db.SetMaxIdleConns(config.DB.MaxIdleConnection)
	db.SetConnMaxLifetime(config.DB.ConnectionLifetime)

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping DB:", err)
	}

	log.Printf("Connected to DB with max open conns: %d", config.DB.MaxOpenConnection)
	log.Printf("Running with log level: %s", config.Logger.LoggerLevel)

	if err := migration.RunMigrations(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	withdrawalRepo := postgresql.NewWithdrawalRepository(db)
	balanceRepo := postgresql.NewBalanceRepository(db)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	withdrawalHandler := handlerhttp.NewWithdrawalHandler(
		service.NewWithdrawalService(withdrawalRepo, balanceRepo),
		config.Token.AuthToken,
	)

	// API routes with auth
	r.Group(func(r chi.Router) {
		r.Use(withdrawalHandler.AuthMiddleware)

		r.Route("/v1/withdrawals", func(r chi.Router) {
			r.Post("/", withdrawalHandler.CreateWithdrawal)
			r.Get("/{id}", withdrawalHandler.GetWithdrawal)
			r.Post("/{id}/confirm", withdrawalHandler.ConfirmWithdrawal)
		})
	})

	// Health checks
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Ready"))
	})


	httpServer := &http.Server{
		Addr:         ":" + config.Server.Port,
		ReadTimeout:  config.Server.ReadTimeout,
		WriteTimeout: config.Server.WriteTimeout,
		IdleTimeout:  config.Server.IdleTimeout,
		Handler:      r,
	}

	go func() {
		log.Printf("Server starting on port %s", config.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")

}
