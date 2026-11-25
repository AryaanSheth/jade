package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/AryaanSheth/jade/internal/jade"
)

func main() {
    cfg, err := jade.ConfigFromEnv()
    if err != nil { log.Fatal(err) }
    port := os.Getenv("PORT")
    if port == "" { port = "8080" }
    server := &http.Server{Addr: ":"+port, Handler: jade.New(cfg, nil), ReadHeaderTimeout: 5*time.Second, ReadTimeout: 10*time.Second, WriteTimeout: 30*time.Second, IdleTimeout: 30*time.Second, MaxHeaderBytes: 16384}
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    go func() { <-ctx.Done(); shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); _ = server.Shutdown(shutdown) }()
    log.Printf("Jade web service listening on :%s", port)
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed { log.Fatal(err) }
}
