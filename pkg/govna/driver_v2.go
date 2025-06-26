// Этот файл содержит реализацию драйвера для семейства NanoVNA V2/LiteVNA (бинарный протокол).
package govna

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/momentics/govna/internal/util"
)

const (
	opNOP        byte = 0x00
	opREAD       byte = 0x10
	opWRITE2     byte = 0x21
	opWRITE4     byte = 0x22
	opREADFIFO   byte = 0x18

	addrSWEEP_START    byte = 0x00
	addrSWEEP_STEP     byte = 0x10
	addrSWEEP_POINTS   byte = 0x20
	addrVALS_FIFO      byte = 0x30
	addrDEVICE_VARIANT byte = 0xf0
)

type V2Driver struct {
	port   util.SerialPortInterface
	config SweepConfig
}

func NewV2Driver(port util.SerialPortInterface) *V2Driver {
	d := &V2Driver{port: port}
	d.resetProtocol()
	return d
}

func (d *V2Driver) resetProtocol() {
	d.port.Write(make([]byte, 8))
}

func (d *V2Driver) Identify() (string, error) {
	d.port.SetReadTimeout(500 * time.Millisecond)
	defer d.port.SetReadTimeout(0)

	if _, err := d.port.Write([]byte{opREAD, addrDEVICE_VARIANT}); err != nil {
		return "", err
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(d.port, buf); err != nil {
		return "", err
	}

	if buf[0] == 2 || buf[0] == 4 { // 2 = V2, 4 = V2Plus4
		return fmt.Sprintf("NanoVNA_V2 (Variant %d)", buf[0]), nil
	}
	return "", errors.New("v2: не является устройством V2")
}

func (d *V2Driver) SetSweep(config SweepConfig) error {
	d.config = config
	step := (config.Stop - config.Start) / float64(config.Points-1)

	if err := d.writeReg64(addrSWEEP_START, uint64(config.Start)); err != nil {
		return err
	}
	if err := d.writeReg64(addrSWEEP_STEP, uint64(step)); err != nil {
		return err
	}
	if err := d.writeReg16(addrSWEEP_POINTS, uint16(config.Points)); err != nil {
		return err
	}
	return nil
}

func (d *V2Driver) Scan() (VNAData, error) {
	if _, err := d.port.Write([]byte{opREADFIFO, addrVALS_FIFO, 0x00}); err != nil {
		return VNAData{}, err
	}

	expectedBytes := d.config.Points * 32
	buf := make([]byte, expectedBytes)
	if _, err := io.ReadFull(d.port, buf); err != nil {
		return VNAData{}, fmt.Errorf("ошибка чтения данных V2: %w", err)
	}
	return d.parseBinaryData(buf)
}

func (d *V2Driver) Close() error { return d.port.Close() }

func (d *V2Driver) parseBinaryData(buf []byte) (VNAData, error) {
	data := VNAData{
		Frequencies: make([]float64, d.config.Points),
		S11:         make([]complex128, d.config.Points),
		S21:         make([]complex128, d.config.Points),
	}
	step := (d.config.Stop - d.config.Start) / float64(d.config.Points-1)

	for i := 0; i < d.config.Points; i++ {
		offset := i * 32
		data.Frequencies[i] = d.config.Start + float64(i)*step
		s11_re := math.Float32frombits(binary.LittleEndian.Uint32(buf[offset : offset+4]))
		s11_im := math.Float32frombits(binary.LittleEndian.Uint32(buf[offset+4 : offset+8]))
		s21_re := math.Float32frombits(binary.LittleEndian.Uint32(buf[offset+16 : offset+20]))
		s21_im := math.Float32frombits(binary.LittleEndian.Uint32(buf[offset+20 : offset+24]))
		data.S11[i] = complex(float64(s11_re), float64(s11_im))
		data.S21[i] = complex(float64(s21_re), float64(s21_im))
	}
	return data, nil
}

func (d *V2Driver) writeReg64(addr byte, val uint64) error {
	buf := make([]byte, 10)
	buf[0] = opWRITE4 + 2
	buf[1] = addr
	binary.LittleEndian.PutUint64(buf[2:], val)
	_, err := d.port.Write(buf)
	return err
}

func (d *V2Driver) writeReg16(addr byte, val uint16) error {
	buf := make([]byte, 4)
	buf[0] = opWRITE2
	buf[1] = addr
	binary.LittleEndian.PutUint16(buf[2:], val)
	_, err := d.port.Write(buf)
	return err
}
