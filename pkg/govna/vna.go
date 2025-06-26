package govna

import (
	"context"
	"errors"
	"fmt"
	"math/cmplx"
	"strings"
	"sync"
	"time"
)

type VNA struct {
	driver Driver
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewVNA(driver Driver) *VNA {
	ctx, cancel := context.WithCancel(context.Background())
	return &VNA{driver: driver, ctx: ctx, cancel: cancel}
}

type SweepConfig struct {
	Start, Stop float64
	Points      int
}

type VNAData struct {
	Frequencies []float64
	S11, S21    []complex128
}

func (v *VNA) SetSweep(config SweepConfig) error {
	if config.Start >= config.Stop || config.Points <= 0 {
		return errors.New("некорректные параметры сканирования")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.driver.SetSweep(config)
}

func (v *VNA) GetData() (VNAData, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.driver.Scan()
}

func (v *VNA) Close() error {
	v.cancel()
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.driver.Close()
}

func (d *VNAData) ToTouchstone() string {
	var sb strings.Builder
	sb.WriteString("! GoVNA Data Export\n")
	sb.WriteString("! Date: " + time.Now().Format(time.RFC3339) + "\n")
	sb.WriteString("# Hz S RI R 50\n")
	for i := range d.Frequencies {
		sb.WriteString(fmt.Sprintf("%d %.6f %.6f %.6f %.6f\n",
			int(d.Frequencies[i]), real(d.S11[i]), imag(d.S11[i]),
			real(d.S21[i]), imag(d.S21[i])))
	}
	return sb.String()
}

func (d *VNAData) CalculateVSWR() []float64 {
	vswr := make([]float64, len(d.S11))
	for i, s11 := range d.S11 {
		gamma := cmplx.Abs(s11)
		if gamma >= 1.0 {
			vswr[i] = 9999.0 // Практически бесконечное значение
		} else {
			vswr[i] = (1 + gamma) / (1 - gamma)
		}
	}
	return vswr
}
