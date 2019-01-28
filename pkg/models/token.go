package models

import (
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
	"gopkg.in/mgo.v2/bson"
	"math/rand"
	"time"
)

type (
	AuthToken struct {
		AccessToken  string `json:"access_token,omitempty"`
		ExpiresIn    int64  `json:"expires_in,omitempty"`
		RefreshToken string `json:"id_token,omitempty"`
	}

	OneTimeTokenService struct {
		Redis    *redis.Client
		Settings *OneTimeTokenSettings
	}

	OneTimeTokenSettings struct {
		Length int
		TTL    int
	}

	OneTimeTokenForm struct {
		ClientId string `json:"client_id" form:"client_id" validate:"required"`
		Token    string `json:"token" form:"token" validate:"required"`
	}

	OneTimeToken struct {
		Token string `json:"token,omitempty"`
	}

	JwtTokenService struct {
		*AuthTokenSettings
	}

	AuthTokenSettings struct {
		JwtKey        []byte
		JwtTTL        int
		JwtMethod     jwt.SigningMethod
		RefreshLength int
		RefreshTTL    int
	}

	JwtClaim struct {
		UserId         bson.ObjectId `json:"user_id"`
		AppId          bson.ObjectId `json:"app_id"`
		Email          string        `json:"email"`
		EmailConfirmed bool          `json:"email_confirmed"`
		Nickname       string        `json:"nickname"`
		jwt.StandardClaims
	}

	RefreshTokenService struct {
		*AuthTokenSettings
	}

	RefreshToken struct {
		Value     string
		TTL       int
		UserAgent string
		IP        string
	}

	RefreshTokenForm struct {
		ClientId string `json:"client_id" form:"client_id" validate:"required"`
		Token    string `json:"token" form:"token" validate:"required"`
	}
)

const (
	OneTimeTokenStoragePattern = "ott_data_%s"
)

var (
	letterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	letterIdxBits = uint(6)
	letterIdxMask = uint64(1<<letterIdxBits - 1)
	letterIdxMax  = 63 / letterIdxBits
	src           = rand.NewSource(time.Now().UnixNano())
)

func NewJwtTokenService(authTokenSettings *AuthTokenSettings) *JwtTokenService {
	return &JwtTokenService{authTokenSettings}
}

func (ts JwtTokenService) Create(user *User) (string, error) {
	claims := &JwtClaim{
		UserId:         user.ID,
		AppId:          user.AppID,
		Email:          user.Email,
		EmailConfirmed: false,
		Nickname:       "",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Unix() + int64(ts.JwtTTL),
			Issuer:    string(user.ID),
			Id:        string(bson.NewObjectId()),
		},
	}

	t := jwt.NewWithClaims(ts.JwtMethod, claims)
	return t.SignedString(ts.JwtKey)
}

func (ts JwtTokenService) Decode(tokenString string) (*JwtClaim, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JwtClaim{}, func(token *jwt.Token) (interface{}, error) {
		return ts.JwtKey, nil
	})

	if claims, ok := token.Claims.(*JwtClaim); ok && token.Valid {
		return claims, nil
	}
	return nil, err
}

func NewRefreshTokenService(authTokenSettings *AuthTokenSettings) *RefreshTokenService {
	return &RefreshTokenService{authTokenSettings}
}

func (rts RefreshTokenService) Create(userAgent string, ip string) *RefreshToken {
	return &RefreshToken{
		Value:     GetRandString(rts.RefreshLength),
		TTL:       rts.RefreshTTL,
		UserAgent: userAgent,
		IP:        ip,
	}
}

func NewOneTimeTokenService(redis *redis.Client, oneTimeTokenSettings *OneTimeTokenSettings) *OneTimeTokenService {
	return &OneTimeTokenService{Redis: redis, Settings: oneTimeTokenSettings}
}

func (os *OneTimeTokenService) Create(s interface{}) (*OneTimeToken, error) {
	t := &OneTimeToken{
		Token: GetRandString(os.Settings.Length),
	}

	s, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	result := os.Redis.Set(fmt.Sprintf(OneTimeTokenStoragePattern, t.Token), s, time.Duration(os.Settings.TTL)*time.Second)
	return t, result.Err()
}

func (os *OneTimeTokenService) Get(token string, d interface{}) error {
	s, err := os.Redis.Get(fmt.Sprintf(OneTimeTokenStoragePattern, token)).Bytes()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(s, &d); err != nil {
		return err
	}
	return nil
}

func (os *OneTimeTokenService) Use(token string, d interface{}) error {
	os.Get(token, &d)
	os.Redis.Del(fmt.Sprintf(OneTimeTokenStoragePattern, token))
	return nil
}

func GetRandString(length int) string {
	b := make([]byte, length)
	for i, cache, remain := length-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(uint64(cache) & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}