package assetpipeline

import "github.com/cwbudde/algo-dsp/dsp/resample"

// Hold the authored endpoints beyond the finite signal instead of injecting
// zeros into the FIR. This preserves nonzero loop endpoints as well as finite
// attacks/tails, without guessing whether a caller will loop the asset. Padding
// is a whole rational input period so trimming adds no fractional phase shift.
func resampleAudioChannel(input []float64, inputRate, outputRate uint32) ([]float64, error) {
	divisor, remainder := inputRate, outputRate
	for remainder != 0 {
		divisor, remainder = remainder, divisor%remainder
	}

	period := int(inputRate / divisor)

	const filterPadding = 64

	padding := ((filterPadding + period - 1) / period) * period
	padded := make([]float64, len(input)+2*padding)
	copy(padded[padding:], input)

	for index := range padding {
		padded[index] = input[0]
		padded[len(padded)-1-index] = input[len(input)-1]
	}

	converted, err := resample.ResampleAligned(padded, float64(inputRate), float64(outputRate), resample.WithQuality(resample.QualityBest))
	if err != nil {
		return nil, err
	}

	start := padding / period * int(outputRate/divisor)
	// Retain the original ceil duration, including sub-frame finite sources.
	frames := int((int64(len(input))*int64(outputRate) + int64(inputRate) - 1) / int64(inputRate))
	if start < 0 || frames > len(converted)-start {
		return nil, ErrAudioBounds
	}

	return converted[start : start+frames], nil
}
