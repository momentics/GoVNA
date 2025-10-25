package govna

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

type CalibrationMethod string

const (
	CalibrationMethodSOL CalibrationMethod = "SOL"
)

type CalibrationStandard string

const (
	CalibrationStandardOpen  CalibrationStandard = "open"
	CalibrationStandardShort CalibrationStandard = "short"
	CalibrationStandardLoad  CalibrationStandard = "load"
	CalibrationStandardThru  CalibrationStandard = "thru"
)

type CalibrationStep struct {
	Standard CalibrationStandard
}

type CalibrationPlan struct {
	Name  string
	Sweep SweepConfig
	Steps []CalibrationStep
}

type CalibrationPrompt func(ctx context.Context, standard CalibrationStandard) error

type CalibrationMeasurement struct {
	Frequencies []float64
	S11         []complex128
	S21         []complex128
}

type CalibrationErrorTerms struct {
	Directivity        []complex128
	SourceMatch        []complex128
	ReflectionTracking []complex128
}

type CalibrationProfile struct {
	Name        string
	Method      CalibrationMethod
	CreatedAt   time.Time
	Sweep       SweepConfig
	Frequencies []float64
	Standards   map[CalibrationStandard]CalibrationMeasurement
	ErrorTerms  CalibrationErrorTerms
}

func (v *VNA) AcquireCalibration(ctx context.Context, plan CalibrationPlan, prompt CalibrationPrompt) (*CalibrationProfile, error) {
	if ctx == nil {
		ctx = v.ctx
	}
	if len(plan.Steps) == 0 {
		return nil, errors.New("план калибровки не содержит шагов")
	}
	if plan.Sweep.Points <= 0 || plan.Sweep.Start >= plan.Sweep.Stop {
		return nil, errors.New("некорректные параметры сканирования в плане калибровки")
	}

	if err := v.SetSweep(plan.Sweep); err != nil {
		return nil, err
	}

	profile := &CalibrationProfile{
		Name:      plan.Name,
		Method:    CalibrationMethodSOL,
		CreatedAt: time.Now(),
		Sweep:     plan.Sweep,
		Standards: make(map[CalibrationStandard]CalibrationMeasurement),
	}

	for _, step := range plan.Steps {
		if prompt != nil {
			if err := prompt(ctx, step.Standard); err != nil {
				return nil, err
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		v.mu.Lock()
		data, err := v.driver.Scan()
		v.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("ошибка получения данных для эталона %s: %w", step.Standard, err)
		}

		profile.Standards[step.Standard] = CalibrationMeasurement{
			Frequencies: cloneFloat64Slice(data.Frequencies),
			S11:         cloneComplexSlice(data.S11),
			S21:         cloneComplexSlice(data.S21),
		}
	}

	if err := profile.computeErrorTerms(); err != nil {
		return nil, err
	}

	if err := profile.Validate(); err != nil {
		return nil, err
	}

	v.mu.Lock()
	v.calibration = profile
	v.mu.Unlock()

	return profile, nil
}

func (p *CalibrationProfile) computeErrorTerms() error {
	openMeas, okOpen := p.Standards[CalibrationStandardOpen]
	shortMeas, okShort := p.Standards[CalibrationStandardShort]
	loadMeas, okLoad := p.Standards[CalibrationStandardLoad]

	if !(okOpen && okShort && okLoad) {
		return errors.New("для расчета коэффициентов SOL требуется набор open/short/load")
	}

	if len(loadMeas.S11) == 0 {
		return errors.New("получены пустые данные калибровки")
	}

	if !frequenciesMatch(loadMeas.Frequencies, openMeas.Frequencies) || !frequenciesMatch(loadMeas.Frequencies, shortMeas.Frequencies) {
		return errors.New("частотные сетки эталонов не совпадают")
	}

	count := len(loadMeas.S11)
	directivity := make([]complex128, count)
	sourceMatch := make([]complex128, count)
	tracking := make([]complex128, count)

	for i := 0; i < count; i++ {
		e00 := loadMeas.S11[i]
		lo := openMeas.S11[i] - e00
		ls := shortMeas.S11[i] - e00
		denom := lo - ls
		if denom == 0 {
			return fmt.Errorf("деление на ноль при расчете коэффициентов на частоте %.3f Гц", loadMeas.Frequencies[i])
		}

		e10e32 := (lo + ls) / denom
		e11 := -ls * (1 + e10e32)

		directivity[i] = e00
		sourceMatch[i] = e11
		tracking[i] = e10e32
	}

	p.Frequencies = cloneFloat64Slice(loadMeas.Frequencies)
	p.ErrorTerms = CalibrationErrorTerms{
		Directivity:        directivity,
		SourceMatch:        sourceMatch,
		ReflectionTracking: tracking,
	}

	return nil
}

func (p *CalibrationProfile) Validate() error {
	if p == nil {
		return errors.New("калибровочный профиль не задан")
	}
	if len(p.Frequencies) == 0 {
		return errors.New("калибровочный профиль не содержит частот")
	}
	if len(p.ErrorTerms.Directivity) != len(p.Frequencies) ||
		len(p.ErrorTerms.SourceMatch) != len(p.Frequencies) ||
		len(p.ErrorTerms.ReflectionTracking) != len(p.Frequencies) {
		return errors.New("коэффициенты калибровки не совпадают по размеру с частотной сеткой")
	}

	if p.Method == CalibrationMethodSOL {
		if _, ok := p.Standards[CalibrationStandardOpen]; !ok {
			return errors.New("отсутствуют измерения эталона open")
		}
		if _, ok := p.Standards[CalibrationStandardShort]; !ok {
			return errors.New("отсутствуют измерения эталона short")
		}
		if _, ok := p.Standards[CalibrationStandardLoad]; !ok {
			return errors.New("отсутствуют измерения эталона load")
		}
	}

	return nil
}

func (p *CalibrationProfile) apply(data VNAData) (VNAData, error) {
	if len(data.Frequencies) != len(p.Frequencies) {
		return VNAData{}, errors.New("размер частотной сетки данных не совпадает с калибровкой")
	}

	for i := range data.Frequencies {
		if math.Abs(data.Frequencies[i]-p.Frequencies[i]) > 1e-3 {
			return VNAData{}, errors.New("частоты данных не совпадают с калибровкой")
		}
	}

	calibrated := VNAData{
		Frequencies: cloneFloat64Slice(data.Frequencies),
		S11:         make([]complex128, len(data.S11)),
		S21:         cloneComplexSlice(data.S21),
	}

	for i, measurement := range data.S11 {
		e00 := p.ErrorTerms.Directivity[i]
		e11 := p.ErrorTerms.SourceMatch[i]
		tracking := p.ErrorTerms.ReflectionTracking[i]

		numerator := measurement - e00
		denominator := e11 + tracking*(measurement-e00)
		if denominator == 0 {
			return VNAData{}, fmt.Errorf("деление на ноль при применении калибровки на частоте %.3f Гц", data.Frequencies[i])
		}
		calibrated.S11[i] = numerator / denominator
	}

	return calibrated, nil
}

func cloneFloat64Slice(src []float64) []float64 {
	if src == nil {
		return nil
	}
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}

func cloneComplexSlice(src []complex128) []complex128 {
	if src == nil {
		return nil
	}
	dst := make([]complex128, len(src))
	copy(dst, src)
	return dst
}

func frequenciesMatch(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-3 {
			return false
		}
	}
	return true
}
