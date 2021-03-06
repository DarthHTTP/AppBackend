/*
 * Copyright (C) 2020  SuperGreenLab <towelie@supergreenlab.com>
 * Author: Constantin Clauzel <constantin.clauzel@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package feeds

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"upper.io/db.v3/lib/sqlbuilder"
)

type publicPlantResult struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ThumbnailPath string `json:"thumbnailPath"`
	FilePath      string `json:"filePath"`
}

type publicPlantsResult struct {
	Plants []publicPlantResult `json:"plants"`
}

func fetchPublicPlants(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit == 0 || limit > 50 {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	plants := []Plant{}
	selector := sess.Select("*")
	selector = selector.From("plants")
	selector = selector.Where("is_public = ?", true)
	selector = selector.OrderBy("cat desc").Offset(offset).Limit(limit)
	if err := selector.All(&plants); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	results := make([]publicPlantResult, 0, len(plants))
	for _, p := range plants {
		selector := sess.Select("*")
		selector = selector.From("feedmedias fm")
		selector = selector.Join("feedentries fe").On("fm.feedentryid = fe.id")
		selector = selector.Join("plants p").On("fe.feedid = p.feedid")
		selector = selector.Where("p.id = ?", p.ID)
		selector = selector.OrderBy("fm.cat desc").Limit(1)
		fm := FeedMedia{}
		if err := selector.One(&fm); err != nil {
			logrus.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fm, err = loadFeedMediaPublicURLs(fm)
		if err != nil {
			logrus.Errorln(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		results = append(results, publicPlantResult{p.ID.UUID.String(), p.Name, fm.FilePath, fm.ThumbnailPath})
	}
	if err := json.NewEncoder(w).Encode(publicPlantsResult{results}); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func fetchPublicPlant(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)

	plant := Plant{}
	if err := sess.Select("*").From("plants").Where("is_public = ?", true).And("id = ?", p.ByName("id")).One(&plant); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(plant); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type publicFeedEntriesResult struct {
	Entries []FeedEntry `json:"entries"`
}

func fetchPublicFeedEntries(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit == 0 || limit > 50 {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	feedEntries := []FeedEntry{}
	selector := sess.Select("fe.*").From("feedentries fe")
	selector = selector.Join("feeds f").On("fe.feedid = f.id")
	selector = selector.Join("plants p").On("p.feedid = f.id")
	selector = selector.Where("p.is_public = ?", true).And("p.id = ?", p.ByName("id")).And("fe.etype not in ('FE_TOWELIE_INFO', 'FE_PRODUCTS')")
	selector = selector.OrderBy("fe.cat DESC").Offset(offset).Limit(limit)
	if err := selector.All(&feedEntries); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(publicFeedEntriesResult{feedEntries}); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type publicFeedMediasResult struct {
	Medias []FeedMedia `json:"medias"`
}

func fetchPublicFeedMedias(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)

	feedMedias := []FeedMedia{}
	selector := sess.Select("fm.*").From("feedmedias fm")
	selector = selector.Join("feedentries fe").On("fm.feedentryid = fe.id")
	selector = selector.Join("feeds f").On("fe.feedid = f.id")
	selector = selector.Join("plants p").On("p.feedid = f.id")
	selector = selector.Where("p.is_public = ?", true).And("fe.id = ?", p.ByName("id"))
	if err := selector.All(&feedMedias); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var err error
	for i, fm := range feedMedias {
		fm, err = loadFeedMediaPublicURLs(fm)
		if err != nil {
			logrus.Errorln(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		feedMedias[i] = fm
	}

	if err := json.NewEncoder(w).Encode(publicFeedMediasResult{feedMedias}); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func fetchPublicFeedMedia(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)

	feedMedia := FeedMedia{}
	selector := sess.Select("fm.*").From("feedmedias fm")
	selector = selector.Join("feedentries fe").On("fm.feedentryid = fe.id")
	selector = selector.Join("feeds f").On("fe.feedid = f.id")
	selector = selector.Join("plants p").On("p.feedid = f.id")
	selector = selector.Where("p.is_public = ?", true).And("fm.id = ?", p.ByName("id"))
	if err := selector.One(&feedMedia); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var err error
	feedMedia, err = loadFeedMediaPublicURLs(feedMedia)
	if err != nil {
		logrus.Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(feedMedia); err != nil {
		logrus.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
