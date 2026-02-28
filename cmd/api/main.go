package main

import (
	"database/sql"
	"log"
	"net/http"

	"idempot/internal/config"
	"idempot/internal/repository/migration"
	"idempot/internal/repository/postgresql"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
)

func main(){
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

	httpServer := &http.Server{
		Addr:         ":" + config.Server.Port,
        ReadTimeout:  config.Server.ReadTimeout,
        WriteTimeout: config.Server.WriteTimeout,
        IdleTimeout:  config.Server.IdleTimeout,
	}

	go func() {
        log.Printf("Server starting on port %s", config.Server.Port)
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

}