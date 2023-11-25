package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ReactionModel struct {
	ID           int64  `db:"id"`
	EmojiName    string `db:"emoji_name"`
	UserID       int64  `db:"user_id"`
	LivestreamID int64  `db:"livestream_id"`
	CreatedAt    int64  `db:"created_at"`
}

type Reaction struct {
	ID         int64      `json:"id"`
	EmojiName  string     `json:"emoji_name"`
	User       User       `json:"user"`
	Livestream Livestream `json:"livestream"`
	CreatedAt  int64      `json:"created_at"`
}

type PostReactionRequest struct {
	EmojiName string `json:"emoji_name"`
}

func getReactionsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	query := "SELECT * FROM reactions WHERE livestream_id = ? ORDER BY created_at DESC"
	if c.QueryParam("limit") != "" {
		limit, err := strconv.Atoi(c.QueryParam("limit"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
		}
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	reactionModels := []ReactionModel{}
	if err := tx.SelectContext(ctx, &reactionModels, query, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "failed to get reactions")
	}

	reactionUserIDs := make([]int64, len(reactionModels))
	reactionUserModels := make([]UserModel, len(reactionModels))
	reactionUserModelsByUserID := make(map[int64]UserModel, len(reactionModels))
	if len(reactionModels) > 0 {
		for i := range reactionModels {
			reactionUserIDs[i] = reactionModels[i].UserID
		}
		query, params, err := sqlx.In("SELECT * FROM users WHERE id IN (?)", reactionUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate query: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &reactionUserModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get users: "+err.Error())
		}
		for i := range reactionUserModels {
			reactionUserModelsByUserID[reactionUserModels[i].ID] = reactionUserModels[i]
		}
	}

	reactionUserThemeModels := make([]ThemeModel, len(reactionUserModels))
	reactionUserThemeModelsByUserID := make(map[int64]ThemeModel, len(reactionUserModels))
	if len(reactionUserIDs) > 0 {
		query, params, err := sqlx.In("SELECT * FROM themes WHERE user_id IN (?)", reactionUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate query: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &reactionUserThemeModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for i := range reactionUserThemeModels {
			reactionUserThemeModelsByUserID[reactionUserThemeModels[i].UserID] = reactionUserThemeModels[i]
		}
	}

	livestreamModel := LivestreamModel{}
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}
	livestreamOwnerModel := UserModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner: "+err.Error())
	}
	livestreamOwnerThemeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner theme: "+err.Error())
	}
	tagModels := []TagModel{}
	if err := tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamModel.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}

	reactions := make([]Reaction, len(reactionModels))
	for i := range reactionModels {
		reaction, err := fillReactionResponse_2(
			reactionModels[i],
			reactionUserModelsByUserID[reactionModels[i].UserID],
			reactionUserThemeModelsByUserID[reactionModels[i].UserID],
			livestreamModel,
			livestreamOwnerModel,
			livestreamOwnerThemeModel,
			tagModels,
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
		}

		reactions[i] = reaction
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, reactions)
}

func postReactionHandler(c echo.Context) error {
	ctx := c.Request().Context()
	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *PostReactionRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	reactionModel := ReactionModel{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		EmojiName:    req.EmojiName,
		CreatedAt:    time.Now().Unix(),
	}

	result, err := tx.NamedExecContext(ctx, "INSERT INTO reactions (user_id, livestream_id, emoji_name, created_at) VALUES (:user_id, :livestream_id, :emoji_name, :created_at)", reactionModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert reaction: "+err.Error())
	}

	reactionID, err := result.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted reaction id: "+err.Error())
	}
	reactionModel.ID = reactionID

	reactionUserModel := UserModel{}
	if err := tx.GetContext(ctx, &reactionUserModel, "SELECT * FROM users WHERE id = ?", reactionModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
	}
	reactionUserThemeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &reactionUserThemeModel, "SELECT * FROM themes WHERE user_id = ?", reactionModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user theme: "+err.Error())
	}
	livestreamModel := LivestreamModel{}
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", reactionModel.LivestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}
	livestreamOwnerModel := UserModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner: "+err.Error())
	}
	livestreamOwnerThemeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner theme: "+err.Error())
	}
	tagModels := []TagModel{}
	if err := tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamModel.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}

	reaction, err := fillReactionResponse_2(
		reactionModel,
		reactionUserModel,
		reactionUserThemeModel,
		livestreamModel,
		livestreamOwnerModel,
		livestreamOwnerThemeModel,
		tagModels,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, reaction)
}

// func fillReactionResponse(ctx context.Context, tx *sqlx.Tx, reactionModel ReactionModel) (Reaction, error) {
// 	userModel := UserModel{}
// 	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE id = ?", reactionModel.UserID); err != nil {
// 		return Reaction{}, err
// 	}
// 	user, err := fillUserResponse(ctx, tx, userModel)
// 	if err != nil {
// 		return Reaction{}, err
// 	}

// 	livestreamModel := LivestreamModel{}
// 	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", reactionModel.LivestreamID); err != nil {
// 		return Reaction{}, err
// 	}
// 	livestreamOwnerModel := UserModel{}
// 	if err := tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
// 		return Reaction{}, err
// 	}
// 	livestreamOwnerThemeModel := ThemeModel{}
// 	if err := tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID); err != nil {
// 		return Reaction{}, err
// 	}
// 	tagModels := []TagModel{}
// 	if err := tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamModel.ID); err != nil {
// 		return Reaction{}, err
// 	}

// 	livestream, err := fillLivestreamResponse_2(
// 		livestreamModel,
// 		livestreamOwnerModel,
// 		livestreamOwnerThemeModel,
// 		tagModels,
// 	)
// 	if err != nil {
// 		return Reaction{}, err
// 	}

// 	reaction := Reaction{
// 		ID:         reactionModel.ID,
// 		EmojiName:  reactionModel.EmojiName,
// 		User:       user,
// 		Livestream: livestream,
// 		CreatedAt:  reactionModel.CreatedAt,
// 	}

// 	return reaction, nil
// }
