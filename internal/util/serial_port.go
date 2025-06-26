// Package util содержит вспомогательные утилиты, не являющиеся частью публичного API.
package util

import (
	"go.bug.st/serial"
	"time"
)

// SerialPortInterface определяет интерфейс для работы с последовательным портом.
// Это позволяет нам использовать реальный порт в production и мок-объект в тестах.
type SerialPortInterface interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close() error
	SetReadTimeout(t time.Duration) error
}

// realPort - это обертка над реальной реализацией последовательного порта.
type realPort struct {
	port serial.Port
}

func (r *realPort) Read(p []byte) (n int, err error)   { return r.port.Read(p) }
func (r *realPort) Write(p []byte) (n int, err error)  { return r.port.Write(p) }
func (r *realPort) Close() error                       { return r.port.Close() }
func (r *realPort) SetReadTimeout(t time.Duration) error { return r.port.SetReadTimeout(t) }

// OpenPort открывает реальный последовательный порт.
func OpenPort(path string, mode *serial.Mode) (SerialPortInterface, error) {
	p, err := serial.Open(path, mode)
	if err != nil {
		return nil, err
	}
	return &realPort{port: p}, nil
}
