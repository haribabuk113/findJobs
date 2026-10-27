// Command locations-import loads the GeoNames location tree into Postgres
// and publishes locations-<UTC>.json. Run once per environment, and again to
// pick up a newer dump:
//
//	dir=$(mktemp -d) && cd "$dir" && for f in countryInfo.txt admin1CodesASCII.txt cities15000.zip; do
//	  curl -fsSO https://download.geonames.org/export/dump/$f; done && cd -
//	DB_PORT=5433 go run ./cmd/locations-import -dir "$dir"
//
// Uses the same DB_* variables as the app.
package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"findJobs/internal/adapters/outbound/geonames"
	"findJobs/internal/adapters/outbound/postgres"
	appreference "findJobs/internal/application/reference"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/logger"
	postgrefacade "findJobs/internal/pkg/postgres"
)

func main() {
	log := logger.Get()
	dir := flag.String("dir", "", "directory holding countryInfo.txt, admin1CodesASCII.txt and cities15000.zip (or .txt)")
	flag.Parse()
	if *dir == "" {
		log.Fatal().Msg("-dir is required")
	}

	countries, err := os.Open(filepath.Join(*dir, "countryInfo.txt"))
	if err != nil {
		log.Fatal().Err(err).Msg("open countryInfo.txt")
	}
	defer countries.Close()
	admin1, err := os.Open(filepath.Join(*dir, "admin1CodesASCII.txt"))
	if err != nil {
		log.Fatal().Err(err).Msg("open admin1CodesASCII.txt")
	}
	defer admin1.Close()
	cities, closeCities, err := openCities(*dir)
	if err != nil {
		log.Fatal().Err(err).Msg("open cities file")
	}
	defer closeCities()

	locations, err := geonames.Parse(countries, admin1, cities)
	if err != nil {
		log.Fatal().Err(err).Msg("parse GeoNames files")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load configuration")
	}
	db, err := postgrefacade.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to PostgreSQL")
	}
	defer postgrefacade.Close()

	// Only the location and asset repositories are used by ImportLocations.
	service := appreference.NewReferenceService(nil, postgres.NewLocationRepository(db), nil, postgres.NewAssetRepository(db), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := service.ImportLocations(ctx, locations); err != nil {
		log.Fatal().Err(err).Msg("import locations")
	}
	log.Info().Int("locations", len(locations)).Msg("locations imported and published")
}

// openCities reads cities15000.zip (the dump's format) or an extracted .txt.
func openCities(dir string) (io.Reader, func(), error) {
	if f, err := os.Open(filepath.Join(dir, "cities15000.txt")); err == nil {
		return f, func() { f.Close() }, nil
	}
	z, err := zip.OpenReader(filepath.Join(dir, "cities15000.zip"))
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range z.File {
		if entry.Name == "cities15000.txt" {
			r, err := entry.Open()
			if err != nil {
				z.Close()
				return nil, nil, err
			}
			// Bound decompression: the real file is ~8 MB.
			return io.LimitReader(r, 64<<20), func() { r.Close(); z.Close() }, nil
		}
	}
	z.Close()
	return nil, nil, fmt.Errorf("cities15000.txt not found in zip")
}
