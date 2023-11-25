package main

import (
	"crypto/sha256"
	"fmt"
)

func fillLivecommentResponse_2(commentOwnerModel UserModel, themeModel ThemeModel, livestreamModel LivestreamModel, livecommentModel LivecommentModel, tagModels []TagModel) (Livecomment, error) {
	commentOwner, err := fillUserResponse_2(commentOwnerModel, themeModel)
	if err != nil {
		return Livecomment{}, err
	}

	livestream, err := fillLivestreamResponse_2(livestreamModel, commentOwnerModel, tagModels, themeModel)
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

func fillLivestreamResponse_2(livestreamModel LivestreamModel, ownerModel UserModel, tagModels []TagModel, themeModel ThemeModel) (Livestream, error) {
	tags := make([]Tag, len(tagModels))
	for i, tagModel := range tagModels {
		tags[i] = Tag{
			ID:   tagModel.ID,
			Name: tagModel.Name,
		}
	}

	owner, err := fillUserResponse_2(ownerModel, themeModel)
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
