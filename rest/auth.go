package rest

import (
	"crypto/des"
	"encoding/base64"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/huskar-t/blm_demo/httperror"
	"github.com/huskar-t/blm_demo/tools"
	"github.com/huskar-t/blm_demo/tools/pool"
	"net/http"
	"strings"
)

var desKey = []byte{
	64,
	182,
	122,
	48,
	86,
	115,
	253,
	68,
}

func DecodeDes(auth string) (user, password string, err error) {
	d, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", err
	}
	if len(d) != 48 {
		return "", "", errors.New("wrong des length")
	}
	block, err := des.NewCipher(desKey)
	if err != nil {
		return "", "", err
	}
	b := pool.BytesPoolGet()
	for i := 0; i < 6; i++ {
		origData := make([]byte, 8)
		block.Decrypt(origData, d[i*8:+(i+1)*8])
		b.Write(origData)
		if i == 2 {
			user = b.String()
			pool.BytesPoolPut(b)
			b = pool.BytesPoolGet()
		}
	}
	password = b.String()
	return user, password, nil
}

func EncodeDes(user, password string) (string, error) {
	if len(user) > 24 || len(password) > 24 {
		return "", errors.New("wrong user or password length")
	}

	b := make([]byte, 48)
	for i := 0; i < len(user); i++ {
		b[i] = user[i]
	}
	for i := 0; i < len(password); i++ {
		b[i+24] = password[i]
	}
	block, err := des.NewCipher(desKey)
	if err != nil {
		return "", err
	}
	buf := pool.BytesPoolGet()
	defer pool.BytesPoolPut(buf)
	for i := 0; i < 6; i++ {
		d := make([]byte, 8)
		block.Encrypt(d, b[i*8:(i+1)*8])
		buf.Write(d)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

const (
	UserKey     = "user"
	PasswordKey = "password"
)

func checkAuth(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if len(auth) == 0 {
		errorResponse(c, httperror.HTTP_NO_AUTH_INFO)
		return
	}
	auth = strings.TrimSpace(auth)
	if strings.HasPrefix(auth, "Basic") {
		user, password, err := tools.DecodeBasic(auth[6:])
		if err != nil {
			errorResponse(c, httperror.HTTP_INVALID_BASIC_AUTH)
			return
		}
		if len(user) == 0 || len(password) == 0 {
			errorResponse(c, httperror.HTTP_INVALID_BASIC_AUTH)
			return
		}
		c.Set(UserKey, user)
		c.Set(PasswordKey, password)
	} else if strings.HasPrefix(auth, "Taosd") {
		user, password, err := DecodeDes(auth[6:])
		if err != nil {
			errorResponse(c, httperror.HTTP_INVALID_BASIC_AUTH)
			return
		}
		if len(user) == 0 || len(password) == 0 {
			errorResponse(c, httperror.HTTP_INVALID_BASIC_AUTH)
			return
		}
		c.Set(UserKey, user)
		c.Set(PasswordKey, password)
	} else {
		errorResponse(c, httperror.HTTP_INVALID_AUTH_TYPE)
		return
	}
}

type Message struct {
	Status string `json:"status"`
	Code   int    `json:"code"`
	Desc   string `json:"desc"`
}

func errorResponse(c *gin.Context, code int) {
	errStr := httperror.ErrorMsgMap[code]
	if len(errStr) == 0 {
		errStr = "unknown error"
	}
	c.AbortWithStatusJSON(http.StatusOK, &Message{
		Status: "error",
		Code:   code,
		Desc:   errStr,
	})
}

func errorResponseWithMsg(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(http.StatusOK, &Message{
		Status: "error",
		Code:   code & 0xffff,
		Desc:   msg,
	})
}
