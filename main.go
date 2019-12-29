package main

import (
	"bufio"
	"encoding/json"
	"github.com/dgryski/go-topk"
	"github.com/kelvins/geocoder"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/uber/h3-go"
	"log"
	"os"
	"strconv"
)

type Activity struct {
	Type       string `json:"type"`
	Confidence int    `json:"confidence"`
}

type ActivityTime struct {
	TimestampMs string     `json:"timestampMs"`
	Activity    []Activity `json:"activity"`
}

type LocationData struct {
	TimestampMs string         `json:"timestampMs"`
	LatitudeE7  int64          `json:"latitudeE7"`
	LongitudeE7 int64          `json:"longitudeE7"`
	Accuracy    int            `json:"accuracy"`
	Activity    []ActivityTime `json:"activity"`
}

type LocationHistory struct {
	Locations []LocationData
}

func main() {

	jww.SetStdoutThreshold(jww.LevelTrace)

	topLocs := topk.New(50)

	file, err := os.Open("locations.json")
	if err != nil {
		log.Fatal(err)
	}

	decoder := json.NewDecoder(bufio.NewReader(file))
	var inp LocationHistory
	err = decoder.Decode(&inp)
	if err != nil {
		jww.ERROR.Println("failed to decode request", err)
		return
	}

	jww.INFO.Println("read locations", len(inp.Locations))

	resolution := 9

	popular := make(map[h3.H3Index]int)

	for _, loc := range inp.Locations {
		geo := h3.GeoCoord{
			Latitude:  float64(loc.LatitudeE7) / 1e7,
			Longitude: float64(loc.LongitudeE7) / 1e7,
		}

		idx := h3.FromGeo(geo, resolution)
		topLocs.Insert(h3.ToString(idx), 1)
		popular[idx] += 1
	}

	jww.INFO.Printf("total hexagons in map = %d, compression %d%%\n", len(popular), 100-len(popular)*100/len(inp.Locations))
	locs := topLocs.Keys()
	for idx, loc := range locs {
		intval, err := strconv.ParseUint(loc.Key, 16, 64)
		if err != nil {
			jww.ERROR.Println("parse error", loc.Key, err)
			break
		}

		gc := h3.ToGeo(h3.H3Index(intval))
		jww.INFO.Println(gc, loc.Count)

		location := geocoder.Location{
			Latitude:  gc.Latitude,
			Longitude: gc.Longitude,
		}
		addresses, err := geocoder.GeocodingReverse(location)
		if err != nil {
			jww.ERROR.Println("reverse geocode failed", err)
			continue
		}

		if len(addresses) > 0 {
			jww.INFO.Println(idx, addresses[0], loc.Count)
		}
	}
}

func sortLocations(popular map[h3.H3Index]int) {
	numIndexes := len(popular)
	locationArray := make([]struct {
		h3Idx h3.H3Index
		count int
	}, numIndexes)

	idx := 0
	maxV := 0
	var maxIdx h3.H3Index
	for k, v := range popular {
		if v > maxV {
			maxV = v
			maxIdx = k
		}
		locationArray[idx].h3Idx = k
		locationArray[idx].count = v
	}

	// TODO: implement sort ops here
	jww.INFO.Printf("most popular location has %d visits\n", maxV)
	jww.INFO.Println(h3.ToGeo(maxIdx))
}
