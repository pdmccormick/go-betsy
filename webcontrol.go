package main

import (
	"./betsy"
	"flag"
	"log"
	"net/http"
	"os"
)

var ifname = flag.String("I", "", "interface name")
var inv_file = flag.String("T", "", "tilemap inventory JSON file")
var bind_addr = flag.String("bind", "localhost", "bind address")
var bind_port = flag.String("port", "3000", "bind port")

func main() {
	flag.Parse()

	if *ifname == "" || *inv_file == "" {
		flag.Usage()
		os.Exit(1)
	}

	app, err := betsy.MakeWebApp(*ifname)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	defer app.Net.Close()

	if err := betsy.LoadTilemapInventory(*inv_file, app.Display); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	addr := *bind_addr + ":" + *bind_port
	log.Printf("Launching control on %s", addr)
	log.Fatal(http.ListenAndServe(addr, app.Router))
}
