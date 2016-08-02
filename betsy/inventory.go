package betsy

import (
	"encoding/json"
	"net"
	"os"
)

type Inventory struct {
	Tilemap []struct {
		IpLinkLocal string    `json:"ipv6_link_local"`
		Start       []float64 `json:"start"`
		Ignore      string    `json:"ignore"`
	} `json:"tilemap"`
}

func LoadTilemapInventory(filename string, disp *Display) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	inv := &Inventory{}
	if err := json.NewDecoder(file).Decode(inv); err != nil {
		return err
	}

	for _, tilemap := range inv.Tilemap {
		if tilemap.Ignore == "true" {
			continue
		}

		ip := net.ParseIP(tilemap.IpLinkLocal)
		tile, err := disp.Net.MakeTile(ip)
		if err != nil {
			return err
		}

		startX := int(tilemap.Start[0])
		startY := int(tilemap.Start[1])

		disp.MapTile(tile, startX, startY)
	}

	disp.SortMapping()

	return nil
}
