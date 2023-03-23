package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ddyachkov/gophermart/internal/config"
	"github.com/ddyachkov/gophermart/internal/handler"
	"github.com/ddyachkov/gophermart/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	flag.Parse()
	cfg := config.DefaultServerConfig()

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(dbCtx, cfg.DatabaseURI)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer dbpool.Close()

	storage, err := storage.NewDBStorage(dbpool, dbCtx)
	if err != nil {
		log.Fatalln(err.Error())
	}

	server := http.Server{
		Addr:    cfg.RunAddress,
		Handler: handler.NewHandler(storage),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("server starting...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-quit

	srvCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(srvCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("server stopped")
}
