// Package main - пример HTTP-сервера для работы с GoVNA.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/momentics/govna/pkg/govna"
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

		// Validate and sanitize port path to prevent path traversal and unauthorized access
		if !isValidPortPath(port) {
			http.Error(w, "Недопустимый путь к порту", http.StatusBadRequest)
			return
		}

		vna, err := pool.Get(port)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка устройства: %v", err), http.StatusInternalServerError)
			return
		}

		// Here parameters can be parsed from the request with validation
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

// isValidPortPath validates that the port path is a proper serial port path
// This prevents path traversal and unauthorized device access
func isValidPortPath(port string) bool {
	if port == "" {
		return false
	}

	// On Unix-like systems, serial ports typically look like:
	// /dev/ttyUSB0, /dev/ttyACM0, /dev/serial/by-id/...
	// On Windows, they look like: COM1, COM2, etc.

	// For Unix/Linux systems
	if len(port) > 4 && port[:5] == "/dev/" {
		// Check that the rest contains only allowed characters
		// and doesn't have path traversal sequences
		remainder := port[5:]
		if strings.Contains(remainder, "../") || strings.Contains(remainder, "..\\") {
			return false
		}
		if strings.Contains(remainder, "/../") || strings.Contains(remainder, "\\..\\") {
			return false
		}
		// Basic check: should not contain null bytes or other control characters
		for _, r := range remainder {
			if r < 32 || r == 127 { // Control characters
				return false
			}
		}
		return true
	}

	// For Windows systems
	if len(port) >= 3 && strings.ToUpper(port[:3]) == "COM" {
		// Should be followed by numbers, e.g., COM1, COM10
		remainder := port[3:]
		_, err := strconv.Atoi(remainder)
		return err == nil
	}

	// Allow other common serial port patterns like /dev/cu.*
	if strings.HasPrefix(port, "/dev/cu.") {
		remainder := strings.TrimPrefix(port, "/dev/cu.")
		if strings.Contains(remainder, "../") || strings.Contains(remainder, "..\\") {
			return false
		}
		for _, r := range remainder {
			if r < 32 || r == 127 {
				return false
			}
		}
		return true
	}

	// Additional check for common serial port patterns like /dev/ttyS*
	if strings.HasPrefix(port, "/dev/ttyS") {
		remainder := strings.TrimPrefix(port, "/dev/ttyS")
		if strings.Contains(remainder, "../") || strings.Contains(remainder, "..\\") {
			return false
		}
		for _, r := range remainder {
			if r < 32 || r == 127 {
				return false
			}
		}
		return true
	}

	// For other patterns, return false to be safe
	return false
}
