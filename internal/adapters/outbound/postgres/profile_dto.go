package postgres

import (
	"context"
	"findJobs/internal/domain/profiles"

	"github.com/jackc/pgx/v5"
)

func CandidateProfileDtoMapper(ctx context.Context, row pgx.Row, profileDetails *profiles.ProfileDetails) error {

	err := row.Scan(&profileDetails.Profile.ID)
	if err != nil {
		// log.ErrorContext(ctx, "Error scanning row:", "error", err)
		return err
	}
	return nil
}
