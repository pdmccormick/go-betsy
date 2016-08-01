package main

import (
	"./betsy"
	"fmt"
	"image"
	"log"
	"net"
	"os"
)

import _ "image/gif"
import _ "image/png"
import _ "image/jpeg"

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

	network, err := betsy.NetworkByInterfaceName("en1")
	if err != nil {
		log.Fatal(err)
	}

	defer network.Conn.Close()

	display := betsy.Display{Net: network}
	tile, err := network.MakeTile(net.ParseIP("ff02::1"))
	if err != nil {
		log.Fatal(err)
	}

	display.MapTile(tile, 0, 0)

	display.SortMapping()

	settings := betsy.DefaultPWMSettings

	err = display.SendFrame(img, settings)
	if err != nil {
		log.Fatal(err)
	}
}
