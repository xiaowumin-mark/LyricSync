package media

import "math"

type spectrumBand struct {
	lower float64
	upper float64
}

type spectrumProcessor struct {
	fftSize        int
	sampleRate     int
	outBands       int
	startFrequency float64
	endFrequency   float64

	filterRadius int
	filterSigma  float64
	kernel       []float64
	kernelSum    float64

	frequencies []float64
	aWeights    []float64
	bands       []spectrumBand
	history     [][]float64
	historyMax  int
	smoothed    []float64

	fft    []complex128
	window []float64
}

func newSpectrumProcessor() *spectrumProcessor {
	return &spectrumProcessor{
		fftSize:        1024,
		startFrequency: 150,
		endFrequency:   4500,
		filterRadius:   2,
		filterSigma:    1,
		historyMax:     5,
	}
}

func (p *spectrumProcessor) Process(samples []float32, outBands, channels, sampleRate int) []float64 {
	if p == nil || outBands <= 0 {
		return nil
	}
	values := make([]float64, outBands)
	if len(samples) == 0 || sampleRate <= 0 {
		return values
	}
	if channels <= 0 {
		channels = 1
	}
	frames := len(samples) / channels
	if frames < 16 {
		return values
	}

	p.configure(sampleRate, outBands)
	p.fillFrequencyData(samples, channels, frames)
	p.gaussianFilter()
	p.timeWeight()
	p.aWeight()
	return p.visualSmooth(p.divide(values))
}

func (p *spectrumProcessor) configure(sampleRate, outBands int) {
	if p.sampleRate == sampleRate && p.outBands == outBands && len(p.fft) == p.fftSize {
		return
	}
	p.sampleRate = sampleRate
	p.outBands = outBands
	p.fft = make([]complex128, p.fftSize)
	p.window = blackmanWindow(p.fftSize)
	p.frequencies = make([]float64, p.fftSize/2)
	p.aWeights = make([]float64, p.fftSize/2)
	p.bands = makeSpectrumBands(p.startFrequency, math.Min(p.endFrequency, float64(sampleRate)/2), outBands)
	p.kernel, p.kernelSum = gaussianKernel(p.filterRadius, p.filterSigma)
	p.history = nil
	p.smoothed = nil

	bandwidth := float64(sampleRate) / float64(p.fftSize)
	for i := range p.aWeights {
		p.aWeights[i] = aWeighting(float64(i) * bandwidth)
	}
}

func (p *spectrumProcessor) fillFrequencyData(samples []float32, channels, frames int) {
	for i := range p.fft {
		p.fft[i] = 0
	}
	windowSize := p.fftSize
	if frames < windowSize {
		windowSize = frames
	}
	startFrame := frames - windowSize
	var windowSum float64
	for n := 0; n < windowSize; n++ {
		frameIndex := startFrame + n
		var mono float64
		for ch := 0; ch < channels; ch++ {
			mono += float64(samples[frameIndex*channels+ch])
		}
		mono /= float64(channels)
		window := p.window[n]
		if windowSize != p.fftSize {
			window = blackmanValue(n, windowSize)
		}
		windowSum += window
		p.fft[n] = complex(mono*window, 0)
	}
	if windowSum <= 0 {
		windowSum = float64(windowSize)
	}

	fft(p.fft)
	for i := range p.frequencies {
		magnitude := 2 * math.Hypot(real(p.fft[i]), imag(p.fft[i])) / windowSum
		p.frequencies[i] = magnitude
	}
}

func (p *spectrumProcessor) gaussianFilter() {
	if p.filterRadius <= 0 || p.kernelSum <= 0 {
		return
	}
	source := append([]float64(nil), p.frequencies...)
	for i := range p.frequencies {
		var sum float64
		for offset := -p.filterRadius; offset <= p.filterRadius; offset++ {
			index := i + offset
			if index < 0 || index >= len(source) {
				continue
			}
			sum += source[index] * p.kernel[offset+p.filterRadius]
		}
		p.frequencies[i] = sum / p.kernelSum
	}
}

func (p *spectrumProcessor) timeWeight() {
	frame := append([]float64(nil), p.frequencies...)
	p.history = append([][]float64{frame}, p.history...)
	if len(p.history) > p.historyMax {
		p.history = p.history[:p.historyMax]
	}
	for i := range p.frequencies {
		var sum float64
		for _, historyFrame := range p.history {
			sum += historyFrame[i]
		}
		p.frequencies[i] = sum / float64(len(p.history))
	}
}

func (p *spectrumProcessor) aWeight() {
	for i := range p.frequencies {
		p.frequencies[i] *= p.aWeights[i]
	}
}

func (p *spectrumProcessor) divide(out []float64) []float64 {
	bandwidth := float64(p.sampleRate) / float64(p.fftSize)
	for i, band := range p.bands {
		startIndex := int(math.Floor(band.lower / bandwidth))
		endIndex := int(math.Floor(band.upper / bandwidth))
		if startIndex < 0 {
			startIndex = 0
		}
		if endIndex >= len(p.frequencies) {
			endIndex = len(p.frequencies) - 1
		}
		if endIndex < startIndex {
			continue
		}

		var sum float64
		for index := startIndex; index <= endIndex; index++ {
			value := p.frequencies[index]
			sum += value * value
		}
		out[i] = visualEnergy(math.Sqrt(sum / float64(endIndex+1-startIndex)))
	}
	return out
}

func (p *spectrumProcessor) visualSmooth(values []float64) []float64 {
	const noiseGate = 0.02
	if len(p.smoothed) != len(values) {
		p.smoothed = make([]float64, len(values))
	}
	for i, next := range values {
		next = clampUnit(next)
		if next < noiseGate {
			next = 0
		}
		prev := p.smoothed[i]
		if next > prev {
			prev = prev*0.35 + next*0.65
		} else {
			prev = prev*0.82 + next*0.18
			if prev < noiseGate*0.6 {
				prev = 0
			}
		}
		p.smoothed[i] = prev
		values[i] = prev
	}
	return values
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func blackmanWindow(size int) []float64 {
	window := make([]float64, size)
	for i := range window {
		window[i] = blackmanValue(i, size)
	}
	return window
}

func blackmanValue(index, size int) float64 {
	if size <= 1 {
		return 1
	}
	ratio := 2 * math.Pi * float64(index) / float64(size-1)
	return 0.42 - 0.5*math.Cos(ratio) + 0.08*math.Cos(2*ratio)
}

func makeSpectrumBands(start, end float64, count int) []spectrumBand {
	bands := make([]spectrumBand, 0, count)
	if count <= 0 {
		return bands
	}
	if start <= 0 {
		start = 1
	}
	if end <= start {
		end = start * 2
	}
	ratio := math.Pow(2, math.Log2(end/start)/float64(count))
	lower := start
	for i := 0; i < count; i++ {
		upper := lower * ratio
		if upper > end || i == count-1 {
			upper = end
		}
		bands = append(bands, spectrumBand{lower: lower, upper: upper})
		lower = upper
	}
	return bands
}

func gaussianKernel(radius int, sigma float64) ([]float64, float64) {
	if radius <= 0 {
		return nil, 0
	}
	if sigma <= 0 {
		sigma = 1
	}
	kernel := make([]float64, radius*2+1)
	var sum float64
	for offset := -radius; offset <= radius; offset++ {
		value := math.Exp(-math.Pow(float64(offset), 2) / (2 * sigma * sigma))
		kernel[offset+radius] = value
		sum += value
	}
	return kernel, sum
}

func aWeighting(frequency float64) float64 {
	f2 := frequency * frequency
	if f2 <= 0 {
		return 0
	}
	return 1.2588966 * 148840000 * f2 * f2 /
		((f2 + 424.36) * math.Sqrt((f2+11599.29)*(f2+544496.41)) * (f2 + 148840000))
}

func fft(values []complex128) {
	n := len(values)
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j &^= bit
		}
		j |= bit
		if i < j {
			values[i], values[j] = values[j], values[i]
		}
	}

	for length := 2; length <= n; length <<= 1 {
		angle := -2 * math.Pi / float64(length)
		wlen := complex(math.Cos(angle), math.Sin(angle))
		half := length / 2
		for i := 0; i < n; i += length {
			w := complex(1, 0)
			for j := 0; j < half; j++ {
				u := values[i+j]
				v := values[i+j+half] * w
				values[i+j] = u + v
				values[i+j+half] = u - v
				w *= wlen
			}
		}
	}
}
