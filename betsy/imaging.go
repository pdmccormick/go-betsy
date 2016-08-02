package betsy

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
)

const TILE_WIDTH = 18
const TILE_HEIGHT = 18
const BYTES_PER_PIXEL = 3 * 2
const FRAME_BUFFER_SIZE = BYTES_PER_PIXEL * TILE_WIDTH * TILE_HEIGHT

type Matrix3x3 [3][3]float32

var Identity3x3 = Matrix3x3{{1.0, 0, 0}, {0, 1.0, 0}, {0, 0, 1.0}}

type PWMSettings struct {
	Gamma      float64
	Postscaler float32
	Transform  Matrix3x3
	GammaLUT   [256]float32
}

func (settings *PWMSettings) SetGamma(gamma float64) {
	settings.Gamma = gamma

	const N = 255
	const Nf float64 = float64(N)
	for i := 0; i <= N; i++ {
		f := float64(i) / Nf
		settings.GammaLUT[i] = float32(math.Pow(f, gamma))
	}
}

var DefaultPWMSettings = &PWMSettings{
	Gamma:      2.4,
	Postscaler: 1.0,
	Transform:  Identity3x3,
}

func init() {
	DefaultPWMSettings.SetGamma(DefaultPWMSettings.Gamma)
}

func (disp *Display) SendFrame(img image.Image, settings *PWMSettings) error {
	buf := make([]byte, FRAME_BUFFER_SIZE)

	for _, m := range disp.Mapping {
		settings.ConvertFrame(img, m.Crop, buf)

		err := m.Tile.SendFrameBuffer(buf)
		if err != nil {
			return err
		}
	}

	return nil
}

func (settings *PWMSettings) ConvertFrame(img image.Image, bounds image.Rectangle, buf []byte) {
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pix := img.At(x, y)
			pwm := settings.ConvertPixel(pix)

			binary.LittleEndian.PutUint16(buf[i:i+2], pwm.R)
			i += 2
			binary.LittleEndian.PutUint16(buf[i:i+2], pwm.G)
			i += 2
			binary.LittleEndian.PutUint16(buf[i:i+2], pwm.B)
			i += 2
		}
	}
}

func (c *PWMSettings) ConvertPixel(pix color.Color) color.RGBA64 {
	Ri, Gi, Bi, _ := pix.RGBA()

	// N (narrow): Range from [0, 65535] into [0, 255]
	const f uint32 = 65535 / 255
	Rn, Gn, Bn := Ri/f, Gi/f, Bi/f

	// G: Apply gamma exponentiation
	Rg := c.GammaLUT[Rn]
	Gg := c.GammaLUT[Gn]
	Bg := c.GammaLUT[Bn]

	// M: Apply channel transformation
	M := &c.Transform
	Rm := Rg*M[0][0] + Gg*M[1][0] + Bg*M[2][0]
	Gm := Rg*M[0][1] + Gg*M[1][1] + Bg*M[2][1]
	Bm := Rg*M[0][2] + Gg*M[1][2] + Bg*M[2][2]

	// P: Apply global post-scaler
	Rp := Rm * c.Postscaler
	Gp := Gm * c.Postscaler
	Bp := Bm * c.Postscaler

	// C: Clamp values to [0, 1] range
	Rc := Rp
	Gc := Gp
	Bc := Bp

	if Rc < 0 {
		Rc = 0
	} else if Rc > 1 {
		Rc = 1
	}

	if Gc < 0 {
		Gc = 0
	} else if Gc > 1 {
		Gc = 1
	}

	if Bc < 0 {
		Bc = 0
	} else if Bc > 1 {
		Bc = 1
	}

	// R: Range from [0, 1] into 16 bit PWM full-scale value
	const PWM_FULL_SCALE float32 = 0x0FFF

	Rr := uint16(Rc * PWM_FULL_SCALE)
	Gr := uint16(Gc * PWM_FULL_SCALE)
	Br := uint16(Bc * PWM_FULL_SCALE)

	return color.RGBA64{
		R: Rr,
		G: Gr,
		B: Br,
		A: 1,
	}
}
