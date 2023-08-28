package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/jinzhu/gorm"
	"go-frame-base/auth0"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

func usernameValidator(f1 validator.FieldLevel) bool {
	if val, ok := f1.Field().Interface().(string); ok {
		match, _ := regexp.MatchString("^[A-Za-z0-9_.]+$", val)
		return match
	}
	return false
}

func (srv *Service) getAuth0AccessToken() string {
	res, err := srv.redis.Get(context.Background(), auth0.CacheKey).Result()
	if err != nil {
		return ""
	}
	return res
}

func (srv *Service) getBrowserUserFromContext(ctx *gin.Context) (*BrowserUser, error) {
	var token string
	if cookieToken, err := ctx.Cookie("token"); err == nil {
		token = cookieToken
	} else {
		return &BrowserUser{}, errors.New("cookie token error")
	}
	//decode jwt
	publicKey := srv.jwtConfig.PublicKey
	publicKey = strings.Replace(publicKey, "||", "\n", -1)
	pb, err := jwt.ParseRSAPublicKeyFromPEM([]byte(publicKey))
	if err != nil {
		return &BrowserUser{}, err
	}
	ret, ok := parse(token, pb)
	if !ok {
		return &BrowserUser{}, errors.New("token parse error:" + err.Error())
	}
	//parse to global struct
	jsonByte, _ := json.Marshal(ret)

	var user BrowserUser

	err = json.Unmarshal(jsonByte, &user)
	if err != nil {
		return &BrowserUser{}, err
	}
	if time.Now().Unix() > user.ExpireTime {
		return &BrowserUser{}, errors.New("forbidden, token has expired, token is: " + token)
	}
	user.Token = token
	return &user, nil
}

func (srv *Service) markUserLoginRedirectUrl(ctx *gin.Context, state string) {
	refer := ctx.Request.Referer()
	redirectUrl := "/loginredirect"

	if refer != "" {
		redirectUrl = refer
	}
	msg := fmt.Sprintf("state: %s, redirect: %s", state, redirectUrl)
	fmt.Println(msg)
	srv.redis.SetEX(ctx, "LoginRedirect:"+state, redirectUrl, time.Hour*24)
}

func (srv *Service) getUserLoginRedirectUrl(ctx *gin.Context, state string) string {
	url, err := srv.redis.Get(ctx, "LoginRedirect:"+state).Result()
	if err != nil {
		return "/loginredirect"
	}
	return url
}

func (srv *Service) generateInviteCode(model interface{}) string {
	var code string
	for true {
		random := GetRandomString(8)
		err := srv.mysql.Client.Where("invite_code = ?", random).Find(model).Error
		if err == gorm.ErrRecordNotFound {
			//code ok
			code = random
			break
		}
	}
	return code
}

func GetRandomString(l int) string {
	str := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}
