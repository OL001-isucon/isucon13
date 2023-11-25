package main

import (
	"crypto/sha256"
	"fmt"
)

func fillLivecommentReportResponse_2(
	reportModel LivecommentReportModel,
	reporterModel UserModel,
	reporterThemeModel ThemeModel,
	livecommentModel LivecommentModel,
	livecommentOwnerModel UserModel,
	livecommentOwnerThemeModel ThemeModel,
	livestreamModel LivestreamModel,
	livestreamOwnerModel UserModel,
	livestreamOwnerThemeModel ThemeModel,
	tagModels []TagModel,
) (LivecommentReport, error) {
	reporter, err := fillUserResponse_2(reporterModel, reporterThemeModel)
	if err != nil {
		return LivecommentReport{}, err
	}

	livecomment, err := fillLivecommentResponse_2(
		livecommentModel,
		livecommentOwnerModel,
		livecommentOwnerThemeModel,
		livestreamModel,
		livestreamOwnerModel,
		livestreamOwnerThemeModel,
		tagModels,
	)
	if err != nil {
		return LivecommentReport{}, err
	}

	report := LivecommentReport{
		ID:          reportModel.ID,
		Reporter:    reporter,
		Livecomment: livecomment,
		CreatedAt:   reportModel.CreatedAt,
	}
	return report, nil
}

func fillLivecommentResponse_2(
	livecommentModel LivecommentModel,
	livecommentOwnerModel UserModel,
	livecommentOwnerThemeModel ThemeModel,
	livestreamModel LivestreamModel,
	livestreamOwnerModel UserModel,
	livestreamOwnerThemeModel ThemeModel,
	tagModels []TagModel,
) (Livecomment, error) {
	commentOwner, err := fillUserResponse_2(livecommentOwnerModel, livecommentOwnerThemeModel)
	if err != nil {
		return Livecomment{}, err
	}

	livestream, err := fillLivestreamResponse_2(livestreamModel, livestreamOwnerModel, livestreamOwnerThemeModel, tagModels)
	if err != nil {
		return Livecomment{}, err
	}

	livecomment := Livecomment{
		ID:         livecommentModel.ID,
		User:       commentOwner,
		Livestream: livestream,
		Comment:    livecommentModel.Comment,
		Tip:        livecommentModel.Tip,
		CreatedAt:  livecommentModel.CreatedAt,
	}

	return livecomment, nil
}

func fillLivestreamResponse_2(livestreamModel LivestreamModel, livestreamOwnerModel UserModel, livestreamOwnerThemeModel ThemeModel, tagModels []TagModel) (Livestream, error) {
	tags := make([]Tag, len(tagModels))
	for i, tagModel := range tagModels {
		tags[i] = Tag{
			ID:   tagModel.ID,
			Name: tagModel.Name,
		}
	}

	owner, err := fillUserResponse_2(livestreamOwnerModel, livestreamOwnerThemeModel)
	if err != nil {
		return Livestream{}, err
	}

	livestream := Livestream{
		ID:           livestreamModel.ID,
		Owner:        owner,
		Title:        livestreamModel.Title,
		Tags:         tags,
		Description:  livestreamModel.Description,
		PlaylistUrl:  livestreamModel.PlaylistUrl,
		ThumbnailUrl: livestreamModel.ThumbnailUrl,
		StartAt:      livestreamModel.StartAt,
		EndAt:        livestreamModel.EndAt,
	}
	return livestream, nil
}

func fillUserResponse_2(userModel UserModel, themeModel ThemeModel) (User, error) {
	// var image []byte
	image, err := getImage(userModel.Name)
	if err != nil {
		return User{}, err
	}

	iconHash := sha256.Sum256(image)

	user := User{
		ID:          userModel.ID,
		Name:        userModel.Name,
		DisplayName: userModel.DisplayName,
		Description: userModel.Description,
		Theme: Theme{
			ID:       themeModel.ID,
			DarkMode: themeModel.DarkMode,
		},
		IconHash: fmt.Sprintf("%x", iconHash),
	}

	return user, nil
}
