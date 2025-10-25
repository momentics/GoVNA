// Этот файл содержит реализацию драйвера для семейства NanoVNA V1 (текстовый протокол).
package govna

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/momentics/govna/internal/util"
)

type V1Driver struct {
	port   util.SerialPortInterface
	config SweepConfig
}

func NewV1Driver(port util.SerialPortInterface) *V1Driver {
	return &V1Driver{port: port}
}

func (d *V1Driver) Identify() (string, error) {
	d.port.SetReadTimeout(500 * time.Millisecond)
	defer d.port.SetReadTimeout(0)

	if _, err := d.port.Write([]byte("version\n")); err != nil {
		return "", fmt.Errorf("v1: ошибка отправки команды version: %w", err)
	}
	reader := bufio.NewReader(d.port)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("v1: не получен ответ на version: %w", err)
	}
	if strings.Contains(strings.ToLower(response), "nanovna") {
		return strings.TrimSpace(response), nil
	}
	return "", errors.New("v1: устройство не опознано как NanoVNA V1")
}

func (d *V1Driver) SetSweep(config SweepConfig) error {
	d.config = config
	cmd := fmt.Sprintf("sweep %d %d %d\n", int(config.Start), int(config.Stop), config.Points)
	_, err := d.port.Write([]byte(cmd))
	return err
}

func (d *V1Driver) Scan() (VNAData, error) {
	if _, err := d.port.Write([]byte("data\n")); err != nil {
		return VNAData{}, err
	}
	time.Sleep(100 * time.Millisecond)
	return d.readData()
}

func (d *V1Driver) Close() error {
	return d.port.Close()
}

func (d *V1Driver) readData() (VNAData, error) {
	data := VNAData{
		Frequencies: make([]float64, 0, d.config.Points),
		S11:         make([]complex128, 0, d.config.Points),
		S21:         make([]complex128, 0, d.config.Points),
	}
	scanner := bufio.NewScanner(d.port)
	for i := 0; i < d.config.Points; i++ {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return data, fmt.Errorf("v1: ошибка чтения строки %d: %w", i+1, err)
			}
			return data, fmt.Errorf("v1: недостаточно данных от устройства (получено %d, ожидалось %d)", i, d.config.Points)
		}
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 5 {
			return data, fmt.Errorf("v1: строка %d содержит %d значений, ожидалось 5: %q", i+1, len(parts), line)
		}
		freq, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return data, fmt.Errorf("v1: не удалось распарсить частоту в строке %d: %w", i+1, err)
		}
		s11Re, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return data, fmt.Errorf("v1: не удалось распарсить действительную часть S11 в строке %d: %w", i+1, err)
		}
		s11Im, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return data, fmt.Errorf("v1: не удалось распарсить мнимую часть S11 в строке %d: %w", i+1, err)
		}
		s21Re, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return data, fmt.Errorf("v1: не удалось распарсить действительную часть S21 в строке %d: %w", i+1, err)
		}
		s21Im, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return data, fmt.Errorf("v1: не удалось распарсить мнимую часть S21 в строке %d: %w", i+1, err)
		}

		data.Frequencies = append(data.Frequencies, freq)
		data.S11 = append(data.S11, complex(s11Re, s11Im))
		data.S21 = append(data.S21, complex(s21Re, s21Im))
	}
	if err := scanner.Err(); err != nil {
		return data, fmt.Errorf("v1: ошибка после чтения данных: %w", err)
	}
	return data, nil
}
