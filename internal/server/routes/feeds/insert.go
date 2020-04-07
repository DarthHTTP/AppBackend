package feeds

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/gofrs/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/rileyr/middleware"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
	"upper.io/db.v3/lib/sqlbuilder"
)

var createUserHandler = insertEndpoint(
	"users",
	func() interface{} { return &User{} },
	[]middleware.Middleware{
		func(fn httprouter.Handle) httprouter.Handle {
			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
				u := r.Context().Value(objectContextKey{}).(*User)
				sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)
				n, err := sess.Collection("users").Find().Where("nickname = ?", u.Nickname).Count()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				if n > 0 {
					http.Error(w, "User already exists", http.StatusBadRequest)
					return
				}

				bc, err := bcrypt.GenerateFromPassword([]byte(u.Password), 8)
				u.Password = string(bc)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fn(w, r, p)
			}
		},
	},
	nil,
)

func fillUserEnd(sess sqlbuilder.Database, ueid uuid.UUID, collection string, all Objects, factory func() UserEndObject) {
	all.Each(func(a Object) {
		ueo := factory()
		ueo.SetUserEndID(ueid)
		ueo.SetObjectID(a.GetID())
		ueo.SetDirty(true)
		sess.Collection(fmt.Sprintf("userend_%s", collection)).Insert(ueo)
	})
}

var createUserEndHandler = insertEndpoint(
	"userends",
	func() interface{} { return &UserEnd{} },
	[]middleware.Middleware{setUserID},
	[]middleware.Middleware{
		func(fn httprouter.Handle) httprouter.Handle {
			return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
				hmacSampleSecret := []byte(viper.GetString("JWTSecret"))
				sess := r.Context().Value(sessContextKey{}).(sqlbuilder.Database)
				id := r.Context().Value(insertedIDContextKey{}).(uuid.UUID)
				uid := r.Context().Value(userIDContextKey{}).(uuid.UUID)

				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"userID":    uid.String(),
					"userEndID": id.String(),
				})
				tokenString, err := token.SignedString(hmacSampleSecret)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.Header().Set("x-sgl-token", tokenString)

				boxes := []Box{}
				sess.Select("*").From("boxes").Where("userid = ?", uid).All(&boxes)
				fillUserEnd(sess, id, "boxes", Boxes(boxes), func() UserEndObject { return &UserEndBox{} })

				plants := []Plant{}
				sess.Select("*").From("plants").Where("userid = ?", uid).All(&plants)
				fillUserEnd(sess, id, "plants", Plants(plants), func() UserEndObject { return &UserEndPlant{} })

				timelapses := []Timelapse{}
				sess.Select("*").From("timelapses").Where("userid = ?", uid).All(&timelapses)
				fillUserEnd(sess, id, "timelapses", Timelapses(timelapses), func() UserEndObject { return &UserEndTimelapse{} })

				devices := []Device{}
				sess.Select("*").From("devices").Where("userid = ?", uid).All(&devices)
				fillUserEnd(sess, id, "devices", Devices(devices), func() UserEndObject { return &UserEndDevice{} })

				feeds := []Feed{}
				sess.Select("*").From("feeds").Where("userid = ?", uid).All(&feeds)
				fillUserEnd(sess, id, "feeds", Feeds(feeds), func() UserEndObject { return &UserEndFeed{} })

				feedEntries := []FeedEntry{}
				sess.Select("*").From("feedentries").Where("userid = ?", uid).All(&feedEntries)
				fillUserEnd(sess, id, "feedentries", FeedEntries(feedEntries), func() UserEndObject { return &UserEndFeedEntry{} })

				feedMedias := []FeedMedia{}
				sess.Select("*").From("feedmedias").Where("userid = ?", uid).All(&feedMedias)
				fillUserEnd(sess, id, "feedmedias", FeedMedias(feedMedias), func() UserEndObject { return &UserEndFeedMedia{} })

				fn(w, r, p)
			}
		},
	},
)

var createBoxHandler = insertEndpoint(
	"boxes",
	func() interface{} { return &Box{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("devices", "DeviceID", true, func() interface{} { return &Device{} }),
	},
	[]middleware.Middleware{
		createUserEndObjects("userend_devices", "DeviceID", func() UserEndObject { return &UserEndDevice{} }),
	},
)

var createPlantHandler = insertEndpoint(
	"plants",
	func() interface{} { return &Plant{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("boxes", "BoxID", false, func() interface{} { return &Box{} }),
	},
	[]middleware.Middleware{
		createUserEndObjects("userend_plants", "PlantID", func() UserEndObject { return &UserEndPlant{} }),
	},
)

var createTimelapseHandler = insertEndpoint(
	"timelapses",
	func() interface{} { return &Timelapse{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("plants", "PlantID", false, func() interface{} { return &Plant{} }),
	},
	[]middleware.Middleware{
		createUserEndObjects("userend_timelapses", "TimelapseID", func() UserEndObject { return &UserEndTimelapse{} }),
	},
)

var createDeviceHandler = insertEndpoint(
	"devices",
	func() interface{} { return &Device{} },
	[]middleware.Middleware{setUserID},
	[]middleware.Middleware{
		createUserEndObjects("userend_devices", "DeviceID", func() UserEndObject { return &UserEndDevice{} }),
	},
)

var createFeedHandler = insertEndpoint(
	"feeds",
	func() interface{} { return &Feed{} },
	[]middleware.Middleware{setUserID},
	[]middleware.Middleware{
		createUserEndObjects("userend_feeds", "FeedID", func() UserEndObject { return &UserEndFeed{} }),
	},
)

var createFeedEntryHandler = insertEndpoint(
	"feedentries",
	func() interface{} { return &FeedEntry{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("feeds", "FeedID", false, func() interface{} { return &Feed{} }),
	},
	[]middleware.Middleware{
		createUserEndObjects("userend_feedentries", "FeedEntryID", func() UserEndObject { return &UserEndFeedEntry{} }),
	},
)

var createFeedMediaHandler = insertEndpoint(
	"feedmedias",
	func() interface{} { return &FeedMedia{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("feedentries", "FeedEntryID", false, func() interface{} { return &FeedEntry{} }),
	},
	[]middleware.Middleware{
		createUserEndObjects("userend_feedmedias", "FeedMediaID", func() UserEndObject { return &UserEndFeedMedia{} }),
	},
)

var createPlantSharingHandler = insertEndpoint(
	"plantsharings",
	func() interface{} { return &PlantSharing{} },
	[]middleware.Middleware{
		setUserID,
		checkAccessRight("feedentries", "FeedEntryID", false, func() interface{} { return &FeedEntry{} }),
	},
	nil,
)
