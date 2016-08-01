package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"net"
	"os"
)

import _ "image/gif"
import _ "image/png"
import _ "image/jpeg"

const PROTOCOL_PORT = 48757

type Matrix3x3 [3][3]float32

var Identity3x3 = Matrix3x3{{1.0, 0, 0}, {0, 1.0, 0}, {0, 0, 1.0}}

type TileConfig struct {
	Width, Height int
	BytesPerPixel int
}

var BetsyTileConfig = &TileConfig{
	Width:         18,
	Height:        18,
	BytesPerPixel: 3 * 2, // RGB * sizeof (uint16)
}

func (cfg *TileConfig) ToCropBox(startX, startY int) image.Rectangle {
	return image.Rect(startX, startY, startX+cfg.Width, startY+cfg.Height)
}

type Network struct {
	Conn          *net.UDPConn
	BroadcastAddr *net.UDPAddr
	Interface     *net.Interface
}

type Tile struct {
	Addr       *net.UDPAddr
	Config     *TileConfig
	FrameBuf   bytes.Buffer
	CommandBuf bytes.Buffer
}

type MappedTile struct {
	Tile
	Display *Display
	Crop    image.Rectangle
}

type Display struct {
	Net   *Network
	Tiles []*MappedTile
}

func (disp *Display) AddTile(ip net.IP, cfg *TileConfig, startX, startY int) (*MappedTile, error) {
	addr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s%%%s]:%d", ip.String(), disp.Net.Interface.Name, PROTOCOL_PORT))
	if err != nil {
		return nil, err
	}

	m := &MappedTile{
		Tile: Tile{
			Addr:   addr,
			Config: cfg,
		},
		Display: disp,
		Crop:    cfg.ToCropBox(startX, startY),
	}

	disp.Tiles = append(disp.Tiles, m)

	return m, nil
}

func NetworkByInterfaceName(name string) (*Network, error) {
	const IPV6_ALL_NODES_MULTICAST_ADDRESS = "ff02::1"

	dev, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	baddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s%%%s]:%d", IPV6_ALL_NODES_MULTICAST_ADDRESS, dev.Name, PROTOCOL_PORT))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp6", nil)
	if err != nil {
		return nil, err
	}

	return &Network{
		Conn:          conn,
		BroadcastAddr: baddr,
		Interface:     dev,
	}, nil
}

type PWMSettings struct {
	Gamma      float64
	Postscaler float32
	Transform  Matrix3x3
}

var DefaultPWMSettings = &PWMSettings{
	Gamma:      2.4,
	Postscaler: 1.0,
	Transform:  Identity3x3,
}

func (disp *Display) SendFrame(img image.Image, settings *PWMSettings) error {
	// TODO: Use channels to do this in parallel
	for _, tile := range disp.Tiles {
		err := tile.ConvertFrame(img, tile.Crop, settings)
		if err != nil {
			return err
		}

		err = tile.SendFrameBuffer(disp.Net.Conn)
		if err != nil {
			return err
		}
	}

	return nil
}

func (tile *Tile) ConvertFrame(img image.Image, bounds image.Rectangle, settings *PWMSettings) error {
	// Transform image to PWM space and pack into framebuffer
	tile.FrameBuf.Reset()
	if err := settings.ConvertFrame(img, bounds, &tile.FrameBuf); err != nil {
		return err
	}

	return nil
}

func (tile *Tile) SendFrameBuffer(conn *net.UDPConn) error {
	// Break frame buffer into chunks and send individually
	const FRAME_CHUNK_SIZE = 1024
	for offs := 0; tile.FrameBuf.Len() > 0; offs += FRAME_CHUNK_SIZE {
		// Extract next chunk of the frame
		chunk := tile.FrameBuf.Next(FRAME_CHUNK_SIZE)

		// Pack chunk into the command buffer
		tile.CommandBuf.Reset()
		fmt.Fprintf(&tile.CommandBuf, "dpc data %d;", offs)
		if _, err := tile.CommandBuf.Write(chunk); err != nil {
			return err
		}

		// Transmit command
		if _, err := conn.WriteTo(tile.CommandBuf.Bytes(), tile.Addr); err != nil {
			return err
		}
	}

	return nil
}

func (settings *PWMSettings) ConvertFrame(img image.Image, bounds image.Rectangle, w io.Writer) error {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pix := img.At(x, y)
			pwm := settings.ConvertPixel(pix)

			if err := binary.Write(w, binary.LittleEndian, pwm.R); err != nil {
				return err
			}

			if err := binary.Write(w, binary.LittleEndian, pwm.G); err != nil {
				return err
			}

			if err := binary.Write(w, binary.LittleEndian, pwm.B); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *PWMSettings) ConvertPixel(pix color.Color) color.RGBA64 {
	// F: Range into [0, 1]
	const RGBA_FULL_SCALE float64 = 65535.0

	Ri, Gi, Bi, _ := pix.RGBA()

	Rf := float64(Ri) / RGBA_FULL_SCALE
	Gf := float64(Gi) / RGBA_FULL_SCALE
	Bf := float64(Bi) / RGBA_FULL_SCALE

	// G: Apply gamma exponentiation
	Rg := float32(math.Pow(Rf, c.Gamma))
	Gg := float32(math.Pow(Gf, c.Gamma))
	Bg := float32(math.Pow(Bf, c.Gamma))

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

func main() {
	file, err := os.Open("image.png")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v image %v\n", format, img.Bounds())

	network, err := NetworkByInterfaceName("en1")
	if err != nil {
		log.Fatal(err)
	}

	defer network.Conn.Close()

	display := Display{Net: network}

	_, err = display.AddTile(net.ParseIP("ff02::1"), BetsyTileConfig, 0, 0)
	if err != nil {
		log.Fatal(err)
	}

	settings := DefaultPWMSettings

	err = display.SendFrame(img, settings)
	if err != nil {
		log.Fatal(err)
	}
}
