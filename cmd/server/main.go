// Package main - пример HTTP-сервера для работы с GoVNA.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourusername/govna/pkg/govna"
)

var (
	scanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "govna_scan_duration_seconds",
			Help: "Duration of VNA scan operations",
		},
		[]string{"port"},
	)
)

func init() {
	prometheus.MustRegister(scanDuration)
}

func main() {
	pool := govna.NewVNAPool()
	defer pool.CloseAll()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scan", scanHandler(pool))
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{Addr: ":8080", Handler: mux}

	go func() {
		log.Println("Сервер запущен на http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка HTTP сервера: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Сервер останавливается...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при корректном завершении сервера: %v", err)
	}
	log.Println("Сервер успешно остановлен.")
}

func scanHandler(pool *govna.VNAPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		port := r.URL.Query().Get("port")
		if port == "" {
			http.Error(w, "Параметр 'port' обязателен", http.StatusBadRequest)
			return
		}

		vna, err := pool.Get(port)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка устройства: %v", err), http.StatusInternalServerError)
			return
		}

		// Здесь можно парсить параметры из запроса
		sweepCfg := govna.SweepConfig{Start: 1e6, Stop: 900e6, Points: 101}
		if err := vna.SetSweep(sweepCfg); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка установки параметров: %v", err), http.StatusInternalServerError)
			return
		}

		start := time.Now()
		data, err := vna.GetData()
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка сканирования: %v", err), http.StatusInternalServerError)
			return
		}
		duration := time.Since(start).Seconds()
		scanDuration.WithLabelValues(port).Observe(duration)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(data.ToTouchstone()))
	}
}
