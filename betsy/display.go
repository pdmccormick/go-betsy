package betsy

import (
	"image"
	"sort"
)

type MappedTile struct {
	Tile    *Tile
	Display *Display
	Crop    image.Rectangle
}

type Display struct {
	Net     *Network
	Mapping []*MappedTile
}

type byRowColumn []*MappedTile

func (s byRowColumn) Len() int {
	return len(s)
}

func (s byRowColumn) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byRowColumn) Less(i, j int) bool {
	if s[i].Crop.Min.Y == s[j].Crop.Min.Y {
		return s[i].Crop.Min.X < s[j].Crop.Min.X
	}

	return s[i].Crop.Min.Y < s[j].Crop.Min.Y
}

func (disp *Display) SortMapping() {
	sort.Sort(byRowColumn(disp.Mapping))
}

func (disp *Display) MapTile(tile *Tile, startX, startY int) *MappedTile {
	m := &MappedTile{
		Tile:    tile,
		Display: disp,
		Crop:    image.Rect(startX, startY, startX+TILE_WIDTH, startY+TILE_HEIGHT),
	}

	disp.Mapping = append(disp.Mapping, m)

	return m
}
