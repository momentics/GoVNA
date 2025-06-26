package govna

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yourusername/govna/internal/util"
)

// MockSerialPort для симуляции ответов устройства
type MockSerialPort struct {
	mu          sync.Mutex
	readBuffer  bytes.Buffer
	writeBuffer bytes.Buffer
}

func (m *MockSerialPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.readBuffer.Read(p)
}
func (m *MockSerialPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeBuffer.Write(p)
}
func (m *MockSerialPort) Close() error                           { return nil }
func (m *MockSerialPort) SetReadTimeout(t time.Duration) error { return nil }
func (m *MockSerialPort) SetReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer.Write(data)
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
	// Ответ V1 должен провалиться, затем V2 должен сработать
	mockPort.SetReadData([]byte("some other device\n")) // Fail V1
	mockPort.SetReadData([]byte{0x02})                   // Succeed V2

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

	expectedS11 := complex(0.5, -0.5)
	// Сравниваем с небольшой погрешностью из-за float32 -> float64
	if cmplx.Abs(data.S11[0]-expectedS11) > 1e-6 {
		t.Errorf("Expected S11 %v, got %v", expectedS11, data.S11[0])
	}
}

func float32ToBytes(f float32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], math.Float32bits(f))
	return buf[:]
}
