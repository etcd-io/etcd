package lipgloss

import (
	"image/color"
	"math"
	"slices"

	"github.com/lucasb-eyer/go-colorful"
)

// Blend1D blends a series of colors together in one linear dimension using multiple
// stops, into the provided number of steps. Uses the "CIE L*, a*, b*" (CIELAB) color-space.
//
// Note that if any of the provided colors are completely transparent, we will
// assume that the alpha value was lost in conversion from RGB -> RGBA, and we
// will set the alpha to opaque, as it's not possible to blend something completely
// transparent.
func Blend1D(steps int, stops ...color.Color) []color.Color {
	if steps < 0 {
		steps = 0
	}

	if steps <= len(stops) {
		return stops[:steps]
	}

	// Ensure they didn't provide any nil colors.
	stops = slices.DeleteFunc(stops, func(c color.Color) bool {
		return c == nil
	})

	if len(stops) == 0 {
		return nil // We can't safely fallback.
	}

	// If they only provided one valid color (or some nil colors), we will just return
	// an array of that color, for the amount of steps they requested.
	if len(stops) == 1 {
		singleColor := stops[0]
		result := make([]color.Color, steps)
		for i := range result {
			result[i] = singleColor
		}
		return result
	}

	blended := make([]color.Color, steps)

	// Convert stops to colorful.Color once
	cstops := make([]colorful.Color, len(stops))
	for i, k := range stops {
		cstops[i], _ = colorful.MakeColor(ensureNotTransparent(k))
	}

	numSegments := len(cstops) - 1
	defaultSize := steps / numSegments
	remainingSteps := steps % numSegments

	resultIndex := 0
	for i := range numSegments {
		from := cstops[i]
		to := cstops[i+1]

		// Calculate segment size.
		segmentSize := defaultSize
		if i < remainingSteps {
			segmentSize++
		}

		divisor := float64(segmentSize - 1)

		// Generate colors for this segment.
		for j := 0; j < segmentSize; j++ {
			var blendingFactor float64
			if segmentSize > 1 {
				blendingFactor = float64(j) / divisor
			}
			blended[resultIndex] = from.BlendLab(to, blendingFactor).Clamped()
			resultIndex++
		}
	}

	return blended
}

// Blend2D blends a series of colors together in two linear dimensions using
// multiple stops, into the provided width/height. Uses the "CIE L*, a*, b*" (CIELAB)
// color-space. The angle parameter controls the rotation of the gradient (0-360°),
// where 0° is left-to-right, 45° is bottom-left to top-right (diagonal). The function
// returns colors in a 1D row-major order ([row1, row2, row3, ...]).
//
// Example of how to iterate over the result:
//
//	gradient := colors.Blend2D(width, height, 180, color1, color2, color3, ...)
//	gradientContent := strings.Builder{}
//	for y := range height {
//		for x := range width {
//			index := y*width + x
//			gradientContent.WriteString(
//				lipgloss.NewStyle().
//					Background(gradient[index]).
//					Render(" "),
//			)
//		}
//		if y < height-1 { // End of row.
//			gradientContent.WriteString("\n")
//		}
//	}
//
// Note that if any of the provided colors are completely transparent, we will
// assume that the alpha value was lost in conversion from RGB -> RGBA, and we
// will set the alpha to opaque, as it's not possible to blend something completely
// transparent.
func Blend2D(width, height int, angle float64, stops ...color.Color) []color.Color {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	// Normalize angle to 0-360.
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}

	// Ensure they didn't provide any nil colors.
	stops = slices.DeleteFunc(stops, func(c color.Color) bool {
		return c == nil
	})

	if len(stops) == 0 {
		return nil // We can't safely fallback.
	}

	// If they only provided one valid color (or some nil colors), we will just return
	// an array of that color, for the amount of pixels they requested.
	if len(stops) == 1 {
		singleColor := stops[0]
		result := make([]color.Color, width*height)
		for i := range result {
			result[i] = singleColor
		}
		return result
	}

	// For 2D blending, we'll create a gradient along the diagonal and then sample
	// from it based on the angle. We'll use the maximum dimension to ensure we have
	// enough resolution for the gradient.
	diagonalGradient := Blend1D(max(width, height), stops...)

	result := make([]color.Color, width*height)

	// Calculate center point for rotation.
	centerX := float64(width-1) / 2.0
	centerY := float64(height-1) / 2.0

	angleRad := angle * math.Pi / 180.0 // -> radians.

	// Pre-calculate sin and cos.
	cosAngle := math.Cos(angleRad)
	sinAngle := math.Sin(angleRad)

	// Calculate diagonal length for proper gradient mapping.
	diagonalLength := math.Sqrt(float64(width*width + height*height))

	// Pre-calculate gradient length for index calculation.
	gradientLen := float64(len(diagonalGradient) - 1)

	for y := range height {
		// Calculate the distance from center along the gradient direction.
		dy := float64(y) - centerY

		for x := 0; x < width; x++ {
			// Calculate the distance from center along the gradient direction.
			dx := float64(x) - centerX

			rotX := dx*cosAngle - dy*sinAngle // Rotate the point by the angle.

			// Map the rotated position to the gradient. Normalize to 0-1 range based on
			// the diagonal length.
			gradientPos := clamp((rotX+diagonalLength/2.0)/diagonalLength, 0, 1)

			// Calculate the index in the gradient.
			gradientIndex := int(gradientPos * gradientLen)
			if gradientIndex >= len(diagonalGradient) {
				gradientIndex = len(diagonalGradient) - 1
			}

			result[y*width+x] = diagonalGradient[gradientIndex] // -> row-major order.
		}
	}

	return result
}
