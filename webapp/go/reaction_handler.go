package main

import (
	"context"
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
	ID         int64      `json:"id" db:"id"`
	EmojiName  string     `json:"emoji_name" db:"emoji_name"`
	User       User       `json:"user" db:"u"`
	Livestream Livestream `json:"livestream" db:"l"`
	CreatedAt  int64      `json:"created_at" db:"created_at"`
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

  query := "SELECT r.id AS id, r.user_id AS user_id, r.livestream_id AS livestream_id, r.emoji_name AS emoji_name, r.created_at AS created_at, u.id AS `u.id`, u.name AS `u.name`, u.display_name AS `u.display_name`, u.password AS `u.password`, u.description AS `u.description`, l.id AS `l.id`, l.user_id AS `l.user_id`, l.title AS `l.title`, l.description AS `l.description`, l.playlist_url AS `l.playlist_url`, l.thumbnail_url AS `l.thumbnail_url`, l.start_at AS `l.start_at`, l.end_at AS `l.end_at` FROM reactions as r JOIN users as u ON u.id=r.user_id JOIN livestreams as l ON l.id=r.livestream_id WHERE livestream_id=7178 ORDER BY r.created_at DESC"
  if c.QueryParam("limit") != "" {
    limit, err := strconv.Atoi(c.QueryParam("limit"))
    if err != nil {
      return echo.NewHTTPError(http.StatusBadRequest, "limit query parameter must be integer")
    }
    query += fmt.Sprintf(" LIMIT %d", limit)
  }

	reactions := []Reaction{}
	if err := tx.SelectContext(ctx, &reactions, query, livestreamID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "failed to get reactions" + err.Error())
	}

	// reactions := make([]Reaction, len(reactionModels))

	// userModel := []UserModel{}
  // var userIdMap = make(map[int64]User)
	// if err := tx.SelectContext(ctx, &userModel, "SELECT * FROM users"); err != nil {
	// 	return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	// }
	// if err != nil {
	// 	return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	// }

  // for _, user := range userModel {
	//   user, err := fillUserResponse(ctx, tx, user)
  //   if err != nil {
  //     return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
  //   }
  //   userIdMap[user.ID] = user
  // }

	// livestreamModel := []LivestreamModel{}
  // var livestreamIdMap = make(map[int64]Livestream)
	// if err := tx.SelectContext(ctx, &livestreamModel, "SELECT * FROM livestreams"); err != nil {
	// 	return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	// }
  // for _, livestream := range livestreamModel {
  //   livestream, err := fillLivestreamResponse(ctx, tx, livestream)
  //   if err != nil {
  //     return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
  //   }
  //   livestreamIdMap[livestream.ID] = livestream
  // }
	// if err := tx.Commit(); err != nil {
	// 	return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	// }

	// for i, reactionModel := range reactionModels {
  //   reaction := Reaction{
  //     ID:         reactionModel.ID,
  //     EmojiName:  reactionModel.EmojiName,
  //     User:       userIdMap[reactionModel.UserID],
  //     Livestream: livestreamIdMap[reactionModel.LivestreamID],
  //     CreatedAt:  reactionModel.CreatedAt,
  //   }

	// 	reactions[i] = reaction
	// }

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

	reaction, err := fillReactionResponse(ctx, tx, reactionModel)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fill reaction: "+err.Error())
	}

	if err := tx.Commit(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit: "+err.Error())
	}

	return c.JSON(http.StatusCreated, reaction)
}

func fillReactionResponse(ctx context.Context, tx *sqlx.Tx, reactionModel ReactionModel) (Reaction, error) {
	userModel := UserModel{}
	if err := tx.GetContext(ctx, &userModel, "SELECT * FROM users WHERE id = ?", reactionModel.UserID); err != nil {
		return Reaction{}, err
	}
	user, err := fillUserResponse(ctx, tx, userModel)
	if err != nil {
		return Reaction{}, err
	}

	livestreamModel := LivestreamModel{}
	if err := tx.GetContext(ctx, &livestreamModel, "SELECT * FROM livestreams WHERE id = ?", reactionModel.LivestreamID); err != nil {
		return Reaction{}, err
	}
	livestream, err := fillLivestreamResponse(ctx, tx, livestreamModel)
	if err != nil {
		return Reaction{}, err
	}

	reaction := Reaction{
		ID:         reactionModel.ID,
		EmojiName:  reactionModel.EmojiName,
		User:       user,
		Livestream: livestream,
		CreatedAt:  reactionModel.CreatedAt,
	}

	return reaction, nil
}
