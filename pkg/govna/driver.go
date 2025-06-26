// Package govna предоставляет API для работы с устройствами NanoVNA.
package govna

import (
	"errors"
	"fmt"
	"go.bug.st/serial"
	"sync"

	"github.com/yourusername/govna/internal/util"
)

// Driver определяет интерфейс, который должен реализовать каждый драйвер устройства.
// Это центральный элемент паттерна "Мост", отделяющий абстракцию от реализации.
type Driver interface {
	Identify() (string, error)
	SetSweep(config SweepConfig) error
	Scan() (VNAData, error)
	Close() error
}

// driverFactory отвечает за определение типа устройства и создание нужного драйвера.
// Она последовательно опрашивает устройство разными драйверами, пока один из них не опознает его.
func driverFactory(port util.SerialPortInterface) (Driver, error) {
	// 1. Попытка идентификации как NanoVNA V1
	v1Driver := NewV1Driver(port)
	if _, err := v1Driver.Identify(); err == nil {
		return v1Driver, nil
	}

	// 2. Попытка идентификации как NanoVNA V2 / LiteVNA
	v2Driver := NewV2Driver(port)
	if _, err := v2Driver.Identify(); err == nil {
		return v2Driver, nil
	}

	return nil, errors.New("не удалось идентифицировать устройство или устройство не поддерживается")
}

// VNAPool управляет пулом VNA устройств для многопоточного доступа.
type VNAPool struct {
	devices map[string]*VNA
	mu      sync.RWMutex
}

func NewVNAPool() *VNAPool { return &VNAPool{devices: make(map[string]*VNA)} }

func (p *VNAPool) Get(portPath string) (*VNA, error) {
	p.mu.RLock()
	if vna, exists := p.devices[portPath]; exists {
		p.mu.RUnlock()
		return vna, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if vna, exists := p.devices[portPath]; exists {
		return vna, nil
	}

	mode := &serial.Mode{BaudRate: 115200}
	port, err := util.OpenPort(portPath, mode)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия порта %s: %w", portPath, err)
	}

	driver, err := driverFactory(port)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("ошибка фабрики драйверов для %s: %w", portPath, err)
	}

	newVNA := NewVNA(driver)
	p.devices[portPath] = newVNA
	return newVNA, nil
}

func (p *VNAPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, vna := range p.devices {
		vna.Close()
	}
}
