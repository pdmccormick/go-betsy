package main

import (
	"./betsy"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"time"
)

import _ "image/gif"
import _ "image/png"
import _ "image/jpeg"

var ifname = flag.String("I", "", "interface name")
var image_fmt = flag.String("F", "", "image path format")
var inv_file = flag.String("T", "", "tilemap inventory JSON file")
var gamma = flag.Float64("G", 2.4, "gamma")
var framerate = flag.Int("R", 30, "framerate")
var postscaler = flag.Float64("P", 0.5, "postscaler")
var start_index = flag.Int("S", 1, "start index (default: 1)")
var stop_index = flag.Int("N", -1, "stop index")
var max_frames = flag.Int("M", -1, "maximum number of frames")

func main() {
	flag.Parse()

	if *ifname == "" || *inv_file == "" || *image_fmt == "" || *stop_index < 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Open network interface
	network, err := betsy.NetworkByInterfaceName(*ifname)
	if err != nil {
		log.Fatal(err)
	}
	defer network.Close()

	// Create display
	display := &betsy.Display{Net: network}

	// Load tilemap
	if err := betsy.LoadTilemapInventory(*inv_file, display); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Set gamma and postscaler
	settings := *betsy.DefaultPWMSettings
	settings.SetGamma(*gamma)
	settings.Postscaler = float32(*postscaler)

	// Load image sequence
	var images []*image.RGBA

	for i := *start_index; i <= *stop_index; i++ {
		filename := fmt.Sprintf(*image_fmt, i)

		file, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
		defer file.Close()

		img, _, err := image.Decode(file)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		images = append(images, img.(*image.RGBA))
	}

	period := time.Second / time.Duration(*framerate)
	c := time.Tick(period)
	num_frames := 0
Loop:
	for j := 0; ; j++ {
		i, n := 0, len(images)
		for now := range c {
			img := images[i]
			err = display.SendFrame(img, &settings)
			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}
			log.Printf("%d/%d/%d: Sent frame in %s", j, i+1, n, time.Since(now))
			i += 1

			display.Net.UploadFrame()
			num_frames++

			if *max_frames > 0 && num_frames >= *max_frames {
				break Loop
			}

			if i >= n {
				break
			}
		}
	}
}
