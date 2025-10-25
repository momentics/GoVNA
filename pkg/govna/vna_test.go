package govna

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/cmplx"
	"strings"

	//	"errors"
	//	"fmt"
	"sync"
	"testing"
	"time"
	//"github.com/momentics/govna/internal/util"
)

// MockSerialPort для симуляции ответов устройства
type MockSerialPort struct {
	mu          sync.Mutex
	readBuffer  bytes.Buffer
	writeBuffer bytes.Buffer
	variant     byte
}

func (m *MockSerialPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readBuffer.Read(p)
}
func (m *MockSerialPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err = m.writeBuffer.Write(p)
	if err == nil && len(p) >= 2 && p[0] == opREAD && p[1] == addrDEVICE_VARIANT && m.variant != 0 {
		m.readBuffer.WriteByte(m.variant)
	}
	return n, err
}
func (m *MockSerialPort) Close() error                         { return nil }
func (m *MockSerialPort) SetReadTimeout(t time.Duration) error { return nil }
func (m *MockSerialPort) SetReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer.Write(data)
}

func (m *MockSerialPort) SetVariant(variant byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.variant = variant
}

// Тестирование фабрики драйверов на выбор V1
func TestDriverFactory_SelectsV1(t *testing.T) {
	mockPort := &MockSerialPort{}
	mockPort.SetReadData([]byte("NanoVNA H\n"))

	driver, err := driverFactory(mockPort)
	if err != nil {
		t.Fatalf("driverFactory failed: %v", err)
	}
	if _, ok := driver.(*V1Driver); !ok {
		t.Fatalf("Expected V1Driver, got %T", driver)
	}
}

// Тестирование фабрики драйверов на выбор V2
func TestDriverFactory_SelectsV2(t *testing.T) {
	mockPort := &MockSerialPort{}
	mockPort.SetVariant(0x02)
	// Ответ V1 должен провалиться, затем V2 должен сработать
	mockPort.SetReadData([]byte("some other device\n"))

	driver, err := driverFactory(mockPort)
	if err != nil {
		t.Fatalf("driverFactory failed: %v", err)
	}
	if _, ok := driver.(*V2Driver); !ok {
		t.Fatalf("Expected V2Driver, got %T", driver)
	}
}

// Тестирование V1Driver на парсинг данных
func TestV1Driver_Scan(t *testing.T) {
	mockPort := &MockSerialPort{}
	driver := NewV1Driver(mockPort)
	cfg := SweepConfig{Points: 1}
	driver.SetSweep(cfg)

	mockPort.SetReadData([]byte("1000000 0.5 -0.5 0.1 -0.1\n"))
	data, err := driver.Scan()
	if err != nil {
		t.Fatalf("V1Driver.Scan failed: %v", err)
	}
	if len(data.S11) != 1 {
		t.Fatalf("Expected 1 point, got %d", len(data.S11))
	}
	expectedS11 := complex(0.5, -0.5)
	if data.S11[0] != expectedS11 {
		t.Errorf("Expected S11 %v, got %v", expectedS11, data.S11[0])
	}
}

func TestV1Driver_ScanInvalidData(t *testing.T) {
	mockPort := &MockSerialPort{}
	driver := NewV1Driver(mockPort)
	cfg := SweepConfig{Points: 1}
	driver.SetSweep(cfg)

	mockPort.SetReadData([]byte("1000000 0.5 oops 0.1 -0.1\n"))
	if _, err := driver.Scan(); err == nil {
		t.Fatalf("expected error while parsing invalid float, got nil")
	}

	mockPort = &MockSerialPort{}
	driver = NewV1Driver(mockPort)
	driver.SetSweep(cfg)
	mockPort.SetReadData([]byte("1000000 0.5\n"))
	if _, err := driver.Scan(); err == nil {
		t.Fatalf("expected error due to insufficient fields, got nil")
	}
}

// Тестирование V2Driver на парсинг бинарных данных
func TestV2Driver_Scan(t *testing.T) {
	mockPort := &MockSerialPort{}
	driver := NewV2Driver(mockPort)
	cfg := SweepConfig{Start: 1e6, Stop: 1e6, Points: 1}
	driver.SetSweep(cfg)

	// Создаем мок-ответ: 1 точка, 32 байта
	var binData bytes.Buffer
	// s11
	binData.Write(float32ToBytes(0.5))
	binData.Write(float32ToBytes(-0.5))
	binData.Write(make([]byte, 8)) // Пропускаем S12
	// s21
	binData.Write(float32ToBytes(0.1))
	binData.Write(float32ToBytes(-0.1))
	binData.Write(make([]byte, 8)) // Пропускаем S22

	mockPort.SetReadData(binData.Bytes())

	data, err := driver.Scan()
	if err != nil {
		t.Fatalf("V2Driver.Scan failed: %v", err)
	}

	if len(data.S11) != 1 {
		t.Fatalf("Expected 1 point, got %d", len(data.S11))
	}

	if math.IsNaN(data.Frequencies[0]) {
		t.Fatalf("Frequency should not be NaN")
	}
	if data.Frequencies[0] != cfg.Start {
		t.Fatalf("Expected frequency %f, got %f", cfg.Start, data.Frequencies[0])
	}

	expectedS11 := complex(0.5, -0.5)
	// Сравниваем с небольшой погрешностью из-за float32 -> float64
	if cmplx.Abs(data.S11[0]-expectedS11) > 1e-6 {
		t.Errorf("Expected S11 %v, got %v", expectedS11, data.S11[0])
	}
}

func TestV2Driver_ScanUnexpectedEOF(t *testing.T) {
	mockPort := &MockSerialPort{}
	driver := NewV2Driver(mockPort)
	cfg := SweepConfig{Start: 1e6, Stop: 1e6, Points: 2}
	driver.SetSweep(cfg)

	// Передаем данные только для одной точки, чтобы спровоцировать ошибку чтения.
	var binData bytes.Buffer
	binData.Write(float32ToBytes(0.5))
	binData.Write(float32ToBytes(-0.5))
	binData.Write(make([]byte, 8))
	binData.Write(float32ToBytes(0.1))
	binData.Write(float32ToBytes(-0.1))
	binData.Write(make([]byte, 8))
	mockPort.SetReadData(binData.Bytes())

	if _, err := driver.Scan(); err == nil {
		t.Fatalf("expected error due to truncated response, got nil")
	}
}

func TestV2Driver_ParseBinaryDataLengthValidation(t *testing.T) {
	driver := &V2Driver{config: SweepConfig{Start: 1e6, Stop: 2e6, Points: 2}}

	invalidBuf := make([]byte, 12)
	if _, err := driver.parseBinaryData(invalidBuf); err == nil {
		t.Fatalf("expected error because buffer size is not multiple of 32")
	}

	shortBuf := make([]byte, 32)
	if _, err := driver.parseBinaryData(shortBuf); err == nil {
		t.Fatalf("expected error because buffer does not match sweep points")
	}
}

func TestVNAData_ToTouchstonePrecision(t *testing.T) {
	data := VNAData{
		Frequencies: []float64{1.23456789e6},
		S11:         []complex128{complex(0.5, -0.5)},
		S21:         []complex128{complex(0.1, -0.1)},
	}

	touchstone := data.ToTouchstone()
	if !strings.Contains(touchstone, "1234567.890000") {
		t.Fatalf("expected frequency to retain fractional part, got %s", touchstone)
	}
}

func float32ToBytes(f float32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(f))
	return buf[:]
}
