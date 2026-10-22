package postgres

// func CandidateProfileDtoMapper(ctx context.Context, row pgx.Row, profileDetails *profiles.ProfileDetails) error {

// 	err := row.Scan(&profileDetails.Profile.ID)
// 	if err != nil {
// 		// log.ErrorContext(ctx, "Error scanning row:", "error", err)
// 		return err
// 	}
// 	return nil
// }

// func (a *API) LoadProfile(ctx context.Context, slug string) (*ProfileResponse, error) {
// 	var (
// 		p                                      ProfileResponse
// 		headlineText                           *string
// 		headlineDesID                          *int64
// 		aboutText                              *string
// 		aboutUpdatedAt                         *time.Time
// 		pronounsJ, socialJ, rolesJ, locsJ, wtJ []byte
// 		timelineJ, skillGroupsJ, workAuthJ     []byte
// 		curAmt, expAmt                         *float64
// 		curCur, expCur                         *string
// 		curHidden                              bool
// 		email                                  string
// 		phone                                  *string
// 		locID                                  *int64
// 		locName                                *string
// 		resumeAccess, resumeShort              *string
// 	)

// 	err := a.DB.QueryRowContext(ctx, profileQuery, slug).Scan(
// 		&p.CandidateID, &p.FullName, &p.ProfileSlug, &p.ProfilePhotoURL, &p.Gender,
// 		&headlineText, &headlineDesID, &aboutText, &aboutUpdatedAt,
// 		&pronounsJ, &socialJ,
// 		&curAmt, &curCur, &curHidden, &expAmt, &expCur,
// 		&rolesJ, &locsJ, &p.Preferences.OpenToRemote, &wtJ,
// 		&timelineJ, &skillGroupsJ,
// 		&email, &phone, &locID, &locName,
// 		&resumeAccess, &resumeShort,
// 		&workAuthJ,
// 	)
// 	if errors.Is(err, sql.ErrNoRows) {
// 		return nil, ErrNotFound
// 	}
// 	if err != nil {
// 		return nil, err
// 	}

// 	// JSON arrays -> typed slices
// 	for _, t := range []struct {
// 		raw []byte
// 		dst any
// 	}{
// 		{pronounsJ, &p.Pronouns},
// 		{socialJ, &p.SocialHandles},
// 		{rolesJ, &p.Preferences.TargetRoles},
// 		{locsJ, &p.Preferences.TargetLocations},
// 		{wtJ, &p.Preferences.WorkTypes},
// 		{timelineJ, &p.Timeline},
// 		{skillGroupsJ, &p.SkillGroups},
// 		{workAuthJ, &p.WorkAuthorization},
// 	} {
// 		if err := json.Unmarshal(t.raw, t.dst); err != nil {
// 			return nil, err
// 		}
// 	}

// 	// headline: null object if neither part is set
// 	if headlineText != nil || headlineDesID != nil {
// 		p.Headline = &Headline{Text: headlineText, DesignationID: headlineDesID}
// 	}

// 	if aboutText != nil {
// 		p.AboutSummary = &AboutSummary{Text: *aboutText, UpdatedAt: aboutUpdatedAt}
// 	}

// 	// salary: current is privacy-gated, expected is always public
// 	p.Salary.Current.IsHidden = curHidden
// 	if !curHidden && curAmt != nil {
// 		p.Salary.Current.Amount = curAmt
// 		p.Salary.Current.Currency = curCur
// 	}
// 	if expAmt != nil && expCur != nil {
// 		p.Salary.Expected = &SalaryAmount{Amount: *expAmt, Currency: *expCur}
// 	}

// 	p.Contact = Contact{Email: email, PhoneNumber: phone}
// 	if locID != nil && locName != nil {
// 		p.Contact.PrimaryLocation = &LocationRef{LocationID: *locID, Name: *locName}
// 	}

// 	// resume: links are only exposed when publicly available
// 	if resumeAccess != nil {
// 		res := &Resume{AccessControl: *resumeAccess}
// 		if *resumeAccess == "PUBLICLY_AVAILABLE" {
// 			v := a.BaseURL + "/r/" + p.ProfileSlug
// 			res.ViewURL = &v
// 			if resumeShort != nil {
// 				s := a.ShortLinkBase + *resumeShort
// 				res.PublicShortLink = &s
// 			}
// 		}
// 		p.Resume = res
// 	}

// 	return &p, nil
// }
