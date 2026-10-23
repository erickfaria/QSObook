package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	dbPath := flag.String("db", "", "caminho SQLite (padrão: diretório de dados do usuário)")
	addr := flag.String("addr", "127.0.0.1:8080", "endereço HTTP em loopback")
	zone := flag.String("timezone", "America/Sao_Paulo", "fuso IANA para apresentação")
	tiles := flag.String("map-tiles-url", defaultMapTiles, "URL de tiles com {z}/{x}/{y}; usado apenas com mapa ativado")
	geocoder := flag.String("geocoder-url", defaultGeocoder, "URL de geocodificação; consultas manuais de cidades com mapa ativado")
	flag.Parse()
	host, _, err := net.SplitHostPort(*addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("-addr deve usar um IP de loopback, por exemplo 127.0.0.1:8080")
	}
	if *dbPath == "" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			base = filepath.Join(home, ".local", "share")
		}
		*dbPath = filepath.Join(base, "qsobook", "qsobook.db")
	}
	loc, err := time.LoadLocation(*zone)
	if err != nil {
		return fmt.Errorf("fuso inválido: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := openStore(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer store.db.Close()
	app, err := newApp(store, loc)
	if err != nil {
		return err
	}
	if err := app.configureMap(*tiles, *geocoder); err != nil {
		return err
	}
	server := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}

	log.Printf("QSObook disponível em http://%s (fuso %s)", listener.Addr(), loc)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err = <-serveErr:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			if closeErr := server.Close(); closeErr != nil {
				return fmt.Errorf("encerrar HTTP: %w", errors.Join(shutdownErr, closeErr))
			}
			return fmt.Errorf("encerrar HTTP: %w", shutdownErr)
		}
		err = <-serveErr
	}

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
