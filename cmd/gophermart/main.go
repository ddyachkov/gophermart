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

	"github.com/ddyachkov/gophermart/internal/accrual"
	"github.com/ddyachkov/gophermart/internal/config"
	"github.com/ddyachkov/gophermart/internal/handler"
	"github.com/ddyachkov/gophermart/internal/queue"
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
		log.Fatalln(err.Error())
	}
	defer dbpool.Close()

	storage, err := storage.NewDBStorage(dbCtx, dbpool)
	if err != nil {
		log.Fatalln(err.Error())
	}

	accrualler := accrual.NewAccrualService(cfg.AccrualSystemAddress)
	queue := queue.NewQueue(accrualler, storage)
	server := http.Server{
		Addr:    cfg.RunAddress,
		Handler: handler.NewHandler(storage, queue),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go queue.Start()

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()

	<-quit

	queue.Stop()

	srvCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(srvCtx); err != nil {
		log.Fatalln(err)
	}
}
