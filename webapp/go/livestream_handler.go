package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

type ReserveLivestreamRequest struct {
	Tags         []int64 `json:"tags"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PlaylistUrl  string  `json:"playlist_url"`
	ThumbnailUrl string  `json:"thumbnail_url"`
	StartAt      int64   `json:"start_at"`
	EndAt        int64   `json:"end_at"`
}

type LivestreamViewerModel struct {
	UserID       int64 `db:"user_id" json:"user_id"`
	LivestreamID int64 `db:"livestream_id" json:"livestream_id"`
	CreatedAt    int64 `db:"created_at" json:"created_at"`
}

type LivestreamModel struct {
	ID           int64  `db:"id" json:"id"`
	UserID       int64  `db:"user_id" json:"user_id"`
	Title        string `db:"title" json:"title"`
	Description  string `db:"description" json:"description"`
	PlaylistUrl  string `db:"playlist_url" json:"playlist_url"`
	ThumbnailUrl string `db:"thumbnail_url" json:"thumbnail_url"`
	StartAt      int64  `db:"start_at" json:"start_at"`
	EndAt        int64  `db:"end_at" json:"end_at"`
}

type Livestream struct {
	ID           int64  `json:"id"`
	Owner        User   `json:"owner"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	PlaylistUrl  string `json:"playlist_url"`
	ThumbnailUrl string `json:"thumbnail_url"`
	Tags         []Tag  `json:"tags"`
	StartAt      int64  `json:"start_at"`
	EndAt        int64  `json:"end_at"`
}

type LivestreamTagModel struct {
	ID           int64 `db:"id" json:"id"`
	LivestreamID int64 `db:"livestream_id" json:"livestream_id"`
	TagID        int64 `db:"tag_id" json:"tag_id"`
}

type ReservationSlotModel struct {
	ID      int64 `db:"id" json:"id"`
	Slot    int64 `db:"slot" json:"slot"`
	StartAt int64 `db:"start_at" json:"start_at"`
	EndAt   int64 `db:"end_at" json:"end_at"`
}

func reserveLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	defer c.Request().Body.Close()

	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var req *ReserveLivestreamRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to decode the request body as json")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	// 2023/11/25 10:00からの１年間の期間内であるかチェック
	var (
		termStartAt    = time.Date(2023, 11, 25, 1, 0, 0, 0, time.UTC)
		termEndAt      = time.Date(2024, 11, 25, 1, 0, 0, 0, time.UTC)
		reserveStartAt = time.Unix(req.StartAt, 0)
		reserveEndAt   = time.Unix(req.EndAt, 0)
	)
	if (reserveStartAt.Equal(termEndAt) || reserveStartAt.After(termEndAt)) || (reserveEndAt.Equal(termStartAt) || reserveEndAt.Before(termStartAt)) {
		return echo.NewHTTPError(http.StatusBadRequest, "bad reservation time range")
	}

	// 予約枠をみて、予約が可能か調べる
	// NOTE: 並列な予約のoverbooking防止にFOR UPDATEが必要
	var slots []*ReservationSlotModel
	if err := tx.SelectContext(ctx, &slots, "SELECT * FROM reservation_slots WHERE start_at >= ? AND end_at <= ? FOR UPDATE", req.StartAt, req.EndAt); err != nil {
		c.Logger().Warnf("予約枠一覧取得でエラー発生: %+v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get reservation_slots: "+err.Error())
	}
	for _, slot := range slots {
		var count int
		if err := tx.GetContext(ctx, &count, "SELECT slot FROM reservation_slots WHERE start_at = ? AND end_at = ?", slot.StartAt, slot.EndAt); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get reservation_slots: "+err.Error())
		}
		c.Logger().Infof("%d ~ %d予約枠の残数 = %d\n", slot.StartAt, slot.EndAt, slot.Slot)
		if count < 1 {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("予約期間 %d ~ %dに対して、予約区間 %d ~ %dが予約できません", termStartAt.Unix(), termEndAt.Unix(), req.StartAt, req.EndAt))
		}
	}

	var (
		livestreamModel = &LivestreamModel{
			UserID:       int64(userID),
			Title:        req.Title,
			Description:  req.Description,
			PlaylistUrl:  req.PlaylistUrl,
			ThumbnailUrl: req.ThumbnailUrl,
			StartAt:      req.StartAt,
			EndAt:        req.EndAt,
		}
	)

	if _, err := tx.ExecContext(ctx, "UPDATE reservation_slots SET slot = slot - 1 WHERE start_at >= ? AND end_at <= ?", req.StartAt, req.EndAt); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update reservation_slot: "+err.Error())
	}

	rs, err := tx.NamedExecContext(ctx, "INSERT INTO livestreams (user_id, title, description, playlist_url, thumbnail_url, start_at, end_at) VALUES(:user_id, :title, :description, :playlist_url, :thumbnail_url, :start_at, :end_at)", livestreamModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream: "+err.Error())
	}

	livestreamID, err := rs.LastInsertId()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get last inserted livestream id: "+err.Error())
	}
	livestreamModel.ID = livestreamID

	// タグ追加
	for _, tagID := range req.Tags {
		if _, err := tx.NamedExecContext(ctx, "INSERT INTO livestream_tags (livestream_id, tag_id) VALUES (:livestream_id, :tag_id)", &LivestreamTagModel{
			LivestreamID: livestreamID,
			TagID:        tagID,
		}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream tag: "+err.Error())
		}
	}

	livestreamOwnerModel := UserModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
	}
	livestreamOwnerThemeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
	}
	tagModels := []TagModel{}
	if err := tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamModel.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}

	livestream, err := fillLivestreamResponse_2(
		*livestreamModel,
		livestreamOwnerModel,
		livestreamOwnerThemeModel,
		tagModels,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livestream: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, livestream)
}

func searchLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	keyTagName := c.QueryParam("tag")

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var livestreamModels []*LivestreamModel
	if c.QueryParam("tag") != "" {
		// タグによる取得
		var tagIDList []int
		if err := tx.SelectContext(ctx, &tagIDList, "SELECT id FROM tags WHERE name = ?", keyTagName); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
		}

		query, params, err := sqlx.In("SELECT * FROM livestream_tags WHERE tag_id IN (?) ORDER BY livestream_id DESC", tagIDList)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to construct IN query: "+err.Error())
		}
		var keyTaggedLivestreams []*LivestreamTagModel
		if err := tx.SelectContext(ctx, &keyTaggedLivestreams, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get keyTaggedLivestreams: "+err.Error())
		}

		for _, keyTaggedLivestream := range keyTaggedLivestreams {
			ls := LivestreamModel{}
			if err := tx.GetContext(ctx, &ls, "SELECT * FROM livestreams WHERE id = ?", keyTaggedLivestream.LivestreamID); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
			}

			livestreamModels = append(livestreamModels, &ls)
		}
	} else {
		// 検索条件なし
		query := `SELECT * FROM livestreams ORDER BY id DESC`
		if c.QueryParam("limit") != "" {
			limit, err := strconv.Atoi(c.QueryParam("limit"))
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
			}
			query += fmt.Sprintf(" LIMIT %d", limit)
		}

		if err := tx.SelectContext(ctx, &livestreamModels, query); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
		}
	}

	livestreamIDs := []int64{}
	for _, livestreamModel := range livestreamModels {
		livestreamIDs = append(livestreamIDs, livestreamModel.ID)
	}

	// owner
	ownerUserIDs := []int64{}
	ownerModels := []UserModel{}
	ownerModelsByUserID := map[int64]UserModel{}
	if len(livestreamModels) > 0 {
		for _, livestreamModel := range livestreamModels {
			ownerUserIDs = append(ownerUserIDs, livestreamModel.UserID)
		}
		query := "SELECT * FROM users WHERE id IN (?)"
		query, params, err := sqlx.In(query, ownerUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &ownerModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, ownerModel := range ownerModels {
			ownerModelsByUserID[ownerModel.ID] = ownerModel
		}
	}

	// theme
	themeModels := []ThemeModel{}
	themeModelsByUserID := map[int64]ThemeModel{}
	if len(ownerUserIDs) > 0 {
		query := "SELECT * FROM themes WHERE user_id IN (?)"
		query, params, err := sqlx.In(query, ownerUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &themeModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, themeModel := range themeModels {
			themeModelsByUserID[themeModel.UserID] = themeModel
		}
	}

	// tag
	tagModels := []TagWithLivestreamIDModel{}
	tagModelsByLivestreamID := map[int64][]TagModel{}
	if len(livestreamIDs) > 0 {
		query := "SELECT tags.id as `id`, tags.name as `name`, livestream_tags.livestream_id as `livestream_id` FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id IN (?)"
		query, params, err := sqlx.In(query, livestreamIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &tagModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
		}
		for _, tagModel := range tagModels {
			tagModelsByLivestreamID[tagModel.LivestreamID] = append(tagModelsByLivestreamID[tagModel.LivestreamID], TagModel{
				ID:   tagModel.ID,
				Name: tagModel.Name,
			})
		}
	}

	livestreams := make([]Livestream, len(livestreamModels))
	for i, livestreamModel := range livestreamModels {
		livestream, err := fillLivestreamResponse_2(
			*livestreamModel,
			ownerModelsByUserID[livestreamModel.UserID],
			themeModelsByUserID[livestreamModel.UserID],
			tagModelsByLivestreamID[livestreamModel.ID],
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livestream: "+err.Error())
		}
		livestreams[i] = livestream
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, livestreams)
}

func getMyLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		return err
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	var livestreamModels []*LivestreamModel
	if err := tx.SelectContext(ctx, &livestreamModels, "SELECT * FROM livestreams WHERE user_id = ?", userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
	}
	livestreams := make([]Livestream, len(livestreamModels))

	livestreamIDs := []int64{}
	for _, livestreamModel := range livestreamModels {
		livestreamIDs = append(livestreamIDs, livestreamModel.ID)
	}

	// owner
	ownerModel := UserModel{}
	query := "SELECT * FROM users WHERE id = ?"
	if err := tx.GetContext(ctx, &ownerModel, query, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
	}

	// theme
	themeModel := ThemeModel{}
	query = "SELECT * FROM themes WHERE user_id = ?"
	if err := tx.GetContext(ctx, &themeModel, query, userID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
	}

	// tag
	tagModels := []TagModel{}
	tagModelsByLivestreamID := map[int64][]TagModel{}
	if len(livestreamIDs) > 0 {
		query := "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id IN (?)"
		query, params, err := sqlx.In(query, livestreamIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &tagModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
		}
		for _, tagModel := range tagModels {
			tagModelsByLivestreamID[tagModel.ID] = append(tagModelsByLivestreamID[tagModel.ID], tagModel)
		}
	}

	for i := range livestreamModels {
		livestream, err := fillLivestreamResponse_2(
			*livestreamModels[i],
			ownerModel,
			themeModel,
			tagModelsByLivestreamID[livestreamModels[i].ID],
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livestream: "+err.Error())
		}
		livestreams[i] = livestream
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, livestreams)
}

func getUserLivestreamsHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		return err
	}

	username := c.Param("username")

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	var user UserModel
	if err := tx.GetContext(ctx, &user, "SELECT * FROM users WHERE name = ?", username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		} else {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user: "+err.Error())
		}
	}

	var livestreamModels []*LivestreamModel
	if err := tx.SelectContext(ctx, &livestreamModels, "SELECT * FROM livestreams WHERE user_id = ?", user.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestreams: "+err.Error())
	}
	livestreams := make([]Livestream, len(livestreamModels))

	livestreamIDs := []int64{}
	for _, livestreamModel := range livestreamModels {
		livestreamIDs = append(livestreamIDs, livestreamModel.ID)
	}

	// owner
	ownerModels := []UserModel{}
	livestreamOwnerModelsByUserID := map[int64]UserModel{}
	if len(livestreamModels) > 0 {
		query := "SELECT * FROM users WHERE id IN (?)"
		query, params, err := sqlx.In(query, livestreamIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &ownerModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, ownerModel := range ownerModels {
			livestreamOwnerModelsByUserID[ownerModel.ID] = ownerModel
		}
	}

	// theme
	themeModels := []ThemeModel{}
	livestreamOwnerThemeModelsByUserID := map[int64]ThemeModel{}
	if len(livestreamIDs) > 0 {
		query := "SELECT * FROM themes WHERE user_id IN (?)"
		query, params, err := sqlx.In(query, livestreamIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &themeModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, themeModel := range themeModels {
			livestreamOwnerThemeModelsByUserID[themeModel.UserID] = themeModel
		}
	}

	// tag
	tagModels := []TagModel{}
	tagModelsByLivestreamID := map[int64][]TagModel{}
	if len(livestreamIDs) > 0 {
		query := "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id IN (?)"
		query, params, err := sqlx.In(query, livestreamIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &tagModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
		}
		for _, tagModel := range tagModels {
			tagModelsByLivestreamID[tagModel.ID] = append(tagModelsByLivestreamID[tagModel.ID], tagModel)
		}
	}

	for i := range livestreamModels {
		livestream, err := fillLivestreamResponse_2(
			*livestreamModels[i],
			livestreamOwnerModelsByUserID[livestreamModels[i].UserID],
			livestreamOwnerThemeModelsByUserID[livestreamModels[i].UserID],
			tagModelsByLivestreamID[livestreamModels[i].ID],
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livestream: "+err.Error())
		}
		livestreams[i] = livestream
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, livestreams)
}

// viewerテーブルの廃止
func enterLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id must be integer")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	viewer := LivestreamViewerModel{
		UserID:       int64(userID),
		LivestreamID: int64(livestreamID),
		CreatedAt:    time.Now().Unix(),
	}

	if _, err := tx.NamedExecContext(ctx, "INSERT INTO livestream_viewers_history (user_id, livestream_id, created_at) VALUES(:user_id, :livestream_id, :created_at)", viewer); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to insert livestream_view_history: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func exitLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()
	if err := verifyUserSession(c); err != nil {
		// echo.NewHTTPErrorが返っているのでそのまま出力
		return err
	}

	// error already checked
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already checked
	userID := sess.Values[defaultUserIDKey].(int64)

	livestreamID, err := strconv.Atoi(c.Param("livestream_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "livestream_id in path must be integer")
	}

	tx, err := dbConn.BeginTxx(ctx, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction: "+err.Error())
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM livestream_viewers_history WHERE user_id = ? AND livestream_id = ?", userID, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete livestream_view_history: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.NoContent(http.StatusOK)
}

func getLivestreamHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
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

	livestreamModel := LivestreamModel{}
	err = tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livestreamID)
	if errors.Is(err, sql.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "not found livestream that has the given id")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}

	livestreamOwnerModel := UserModel{}
	err = tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner: "+err.Error())
	}
	livestreamOwnerThemeModel := ThemeModel{}
	err = tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner theme: "+err.Error())
	}
	tagModels := []TagModel{}
	err = tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}

	livestream, err := fillLivestreamResponse_2(
		livestreamModel,
		livestreamOwnerModel,
		livestreamOwnerThemeModel,
		tagModels,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livestream: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, livestream)
}

func getLivecommentReportsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	if err := verifyUserSession(c); err != nil {
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

	var livestreamModel LivestreamModel
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream: "+err.Error())
	}

	// error already check
	sess, _ := session.Get(defaultSessionIDKey, c)
	// existence already check
	userID := sess.Values[defaultUserIDKey].(int64)

	if livestreamModel.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "can't get other streamer's livecomment reports")
	}

	var reportModels []*LivecommentReportModel
	if err := tx.SelectContext(ctx, &reportModels, "SELECT * FROM livecomment_reports WHERE livestream_id = ?", livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomment reports: "+err.Error())
	}

	reportUserIDs := []int64{}
	for _, reportModel := range reportModels {
		reportUserIDs = append(reportUserIDs, reportModel.UserID)
	}

	reportLivecommentIDs := []int64{}
	for _, reportModel := range reportModels {
		reportLivecommentIDs = append(reportLivecommentIDs, reportModel.LivecommentID)
	}

	// reporter
	reporterModels := []UserModel{}
	reporterModelsByUserID := map[int64]UserModel{}
	if len(reportModels) > 0 {
		query := "SELECT * FROM users WHERE id IN (?)"
		query, params, err := sqlx.In(query, reportUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &reporterModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, reporterModel := range reporterModels {
			reporterModelsByUserID[reporterModel.ID] = reporterModel
		}
	}

	// reporter theme
	reporterThemeModels := []ThemeModel{}
	reporterThemeModelsByUserID := map[int64]ThemeModel{}
	if len(reportUserIDs) > 0 {
		query := "SELECT * FROM themes WHERE user_id IN (?)"
		query, params, err := sqlx.In(query, reportUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &reporterThemeModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, reporterThemeModel := range reporterThemeModels {
			reporterThemeModelsByUserID[reporterThemeModel.UserID] = reporterThemeModel
		}
	}

	// livecomment
	livecommentModels := []LivecommentModel{}
	livecommentModelsByReportID := map[int64]LivecommentModel{}
	if len(reportLivecommentIDs) > 0 {
		query := "SELECT * FROM livecomments WHERE id IN (?)"
		query, params, err := sqlx.In(query, reportLivecommentIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &livecommentModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livecomments: "+err.Error())
		}
		for _, livecommentModel := range livecommentModels {
			livecommentModelsByReportID[livecommentModel.ID] = livecommentModel
		}
	}

	// livecomment owner
	livecommentOwnerUserIDs := []int64{}
	livecommentOwnerModels := []UserModel{}
	livecommentOwnerModelsByUserID := map[int64]UserModel{}
	if len(livecommentModels) > 0 {
		for _, livecommentModel := range livecommentModels {
			livecommentOwnerUserIDs = append(livecommentOwnerUserIDs, livecommentModel.UserID)
		}
		query := "SELECT * FROM users WHERE id IN (?)"
		query, params, err := sqlx.In(query, livecommentOwnerUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &livecommentOwnerModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, livecommentOwnerModel := range livecommentOwnerModels {
			livecommentOwnerModelsByUserID[livecommentOwnerModel.ID] = livecommentOwnerModel
		}
	}

	// livecomment owner theme
	livecommentOwnerThemeModels := []ThemeModel{}
	livecommentOwnerThemeModelsByUserID := map[int64]ThemeModel{}
	if len(livecommentOwnerUserIDs) > 0 {
		query := "SELECT * FROM themes WHERE user_id IN (?)"
		query, params, err := sqlx.In(query, livecommentOwnerUserIDs)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate sql by sqlx.In: "+err.Error())
		}
		if err := tx.SelectContext(ctx, &livecommentOwnerThemeModels, query, params...); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get themes: "+err.Error())
		}
		for _, livecommentOwnerThemeModel := range livecommentOwnerThemeModels {
			livecommentOwnerThemeModelsByUserID[livecommentOwnerThemeModel.UserID] = livecommentOwnerThemeModel
		}
	}

	livestreamOwnerModel := UserModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner: "+err.Error())
	}
	livestreamOwnerThemeModel := ThemeModel{}
	if err := tx.GetContext(ctx, &livestreamOwnerThemeModel, "SELECT * FROM themes WHERE user_id = ?", livestreamModel.UserID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get livestream owner theme: "+err.Error())
	}

	// livestream tags
	livestreamTags := []TagModel{}
	query := "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?"
	if err := tx.SelectContext(ctx, &livestreamTags, query, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tags: "+err.Error())
	}

	reports := make([]LivecommentReport, len(reportModels))
	for i := range reportModels {
		report, err := fillLivecommentReportResponse_2(
			*reportModels[i],
			reporterModelsByUserID[reportModels[i].UserID],
			reporterThemeModelsByUserID[reportModels[i].UserID],
			livecommentModelsByReportID[reportModels[i].LivecommentID],
			livecommentOwnerModelsByUserID[livecommentModelsByReportID[reportModels[i].LivecommentID].UserID],
			livecommentOwnerThemeModelsByUserID[livecommentModelsByReportID[reportModels[i].LivecommentID].UserID],
			livestreamModel,
			livestreamOwnerModel,
			livestreamOwnerThemeModel,
			livestreamTags,
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill livecomment report: "+err.Error())
		}
		reports[i] = report
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusOK, reports)
}

// func fillLivestreamResponse(ctx context.Context, tx *sqlx.Tx, livestreamModel LivestreamModel) (Livestream, error) {
// 	ownerModel := UserModel{}
// 	if err := tx.GetContext(ctx, &ownerModel, "SELECT * FROM users WHERE id = ?", livestreamModel.UserID); err != nil {
// 		return Livestream{}, err
// 	}
// 	owner, err := fillUserResponse(ctx, tx, ownerModel)
// 	if err != nil {
// 		return Livestream{}, err
// 	}

// 	tagModels := []TagModel{}
// 	if err := tx.SelectContext(ctx, &tagModels, "SELECT tags.* FROM tags JOIN livestream_tags ON livestream_tags.tag_id = tags.id WHERE livestream_tags.livestream_id = ?", livestreamModel.ID); err != nil {
// 		return Livestream{}, err
// 	}

// 	tags := make([]Tag, len(tagModels))
// 	for i, tagModel := range tagModels {
// 		tags[i] = Tag{
// 			ID:   tagModel.ID,
// 			Name: tagModel.Name,
// 		}
// 	}

// 	livestream := Livestream{
// 		ID:           livestreamModel.ID,
// 		Owner:        owner,
// 		Title:        livestreamModel.Title,
// 		Tags:         tags,
// 		Description:  livestreamModel.Description,
// 		PlaylistUrl:  livestreamModel.PlaylistUrl,
// 		ThumbnailUrl: livestreamModel.ThumbnailUrl,
// 		StartAt:      livestreamModel.StartAt,
// 		EndAt:        livestreamModel.EndAt,
// 	}
// 	return livestream, nil
// }
