package clientselector

import (
	"math"
	"net/url"
	"strconv"
)

func getClosestServer(lat1, lon1 float64, servers map[string]string) []string {
	var server []string
	min := math.MaxFloat64

	for s, metadata := range servers {
		if v, err := url.ParseQuery(metadata); err == nil {
			lat2Str := v.Get("latitude")
			lon2Str := v.Get("longitude")

			if lat2Str == "" || lon2Str == "" {
				continue
			}

			lat2, err := strconv.ParseFloat(lat2Str, 64)
			if err != nil {
				continue
			}
			lon2, err := strconv.ParseFloat(lon2Str, 64)
			if err != nil {
				continue
			}

			d := getDistanceFrom(lat1, lon1, lat2, lon2)
			if d < min {
				server = []string{s}
				min = d
			} else if d == min {
				server = append(server, s)
			}

		}
	}
	return server
}

//https://gist.github.com/cdipaolo/d3f8db3848278b49db68
func getDistanceFrom(lat1, lon1, lat2, lon2 float64) float64 {
	var la1, lo1, la2, lo2, r float64
	la1 = lat1 * math.Pi / 180
	lo1 = lon1 * math.Pi / 180
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	r = 6378100 // Earth radius in METERS

	// calculate
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)

	return 2 * r * math.Asin(math.Sqrt(h))
}

func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}
