// Package geonames parses the GeoNames dump files (CC-BY 4.0; attribution on
// the About page) into the country -> region -> city tree:
// countryInfo.txt, admin1CodesASCII.txt and citiesN.txt.
package geonames

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"findJobs/internal/domain/reference"
)

var (
	geonameID   = regexp.MustCompile(`^[1-9][0-9]{0,11}$`)
	countryCode = regexp.MustCompile(`^[A-Z]{2}$`)
)

// Parse builds the tree, parents before children (the order the importer
// inserts in). Rows with unknown parents or invalid fields are skipped;
// malformed files fail.
func Parse(countries, admin1, cities io.Reader) ([]reference.Location, error) {
	var out []reference.Location
	countryIDs := map[string]string{} // ISO code -> geoname id

	err := eachRow(countries, 17, func(f []string) error {
		code, name, id := f[0], f[4], f[16]
		if !countryCode.MatchString(code) || !geonameID.MatchString(id) || !validName(name) {
			return nil
		}
		countryIDs[code] = id
		out = append(out, reference.Location{ID: id, Level: "country", Name: name, CountryCode: code})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("countryInfo: %w", err)
	}

	regionIDs := map[string]string{} // "IN.19" -> geoname id
	err = eachRow(admin1, 4, func(f []string) error {
		key, name, id := f[0], f[1], f[3]
		cc, _, _ := strings.Cut(key, ".")
		parent, ok := countryIDs[cc]
		if !ok || !geonameID.MatchString(id) || !validName(name) {
			return nil
		}
		regionIDs[key] = id
		out = append(out, reference.Location{ID: id, ParentID: &parent, Level: "region", Name: name, CountryCode: cc})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("admin1: %w", err)
	}

	seen := map[string]bool{}
	for _, l := range out {
		seen[l.ID] = true
	}
	err = eachRow(cities, 15, func(f []string) error {
		id, name, cc, adm1 := f[0], f[1], f[8], f[10]
		lat, errLat := strconv.ParseFloat(f[4], 64)
		lng, errLng := strconv.ParseFloat(f[5], 64)
		if !geonameID.MatchString(id) || seen[id] || !validName(name) || errLat != nil || errLng != nil ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			return nil
		}
		// A city hangs under its region when GeoNames has one, else its country.
		parent, ok := regionIDs[cc+"."+adm1]
		if !ok {
			if parent, ok = countryIDs[cc]; !ok {
				return nil
			}
		}
		seen[id] = true
		out = append(out, reference.Location{ID: id, ParentID: &parent, Level: "city", Name: name, CountryCode: cc, Lat: &lat, Lng: &lng})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cities: %w", err)
	}
	return out, nil
}

// eachRow calls fn for every non-comment tab-separated row with at least
// minFields fields.
func eachRow(r io.Reader, minFields int, fn func([]string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1<<20) // cities rows carry long alternate-name lists
	rows := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < minFields {
			return fmt.Errorf("row %d has %d fields, want at least %d", rows+1, len(f), minFields)
		}
		if err := fn(f); err != nil {
			return err
		}
		rows++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no rows")
	}
	return nil
}

func validName(s string) bool {
	if s == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 200 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}
