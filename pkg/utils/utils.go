package utils

import (
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"strconv"
	"time"

	"sgin/pkg/config"

	jwt "github.com/golang-jwt/jwt/v4"
)

// 从配置加载 JWT 秘钥，若未配置则回退到 PasswdKey
func jwtSecret() []byte {
	cfg := config.GetConfig()
	if cfg != nil && cfg.PasswdKey != "" {
		return []byte(cfg.PasswdKey)
	}
	// 最小化改动：保持兼容旧行为
	return []byte("your-secret-key")
}

// GenerateToken 生成 JWT token
func GenerateToken(userID string) (string, error) {
	// 定义 JWT 的有效期限
	expirationTime := time.Now().Add(24 * time.Hour)

	// 创建 token 的声明部分
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expirationTime.Unix(),
	}

	// 使用 HS256 算法进行签名
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(jwtSecret())
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// ParseToken 解析 JWT token
func ParseToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// 解析token返回user_id
func ParseTokenGetUserID(tokenString string) (string, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return "", err
	}
	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", errors.New("user_id not found in token")
	}
	return userID, nil
}

// 生成验证码
func GenerateVerificationCode() string {
	const n = 6
	max := 1000000
	var num int
	var buf [4]byte
	if _, err := crand.Read(buf[:]); err == nil {
		num = int((uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])) % uint32(max))
	} else {
		rand.Seed(time.Now().UnixNano())
		num = rand.Intn(max)
	}
	s := strconv.Itoa(num)
	for len(s) < n {
		s = "0" + s
	}
	return s
}

// SignBody 签名
func SignBody(body, secretKey []byte) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// 数组转换成json字符串
func ArrayToJsonString(arr []string) string {
	if len(arr) == 0 {
		return ""
	}
	jsonBytes, _ := json.Marshal(arr)
	return string(jsonBytes)
}

func MapGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

func MapGetFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// GetFileMd5
func GetFileMd5(fileinfo multipart.File) (string, error) {
	md5h := md5.New()
	if _, err := io.Copy(md5h, fileinfo); err != nil {
		return "", err
	}
	return hex.EncodeToString(md5h.Sum(nil)), nil
}

// GenerateOrderID generates a unique order ID based on the current date and time including nanoseconds.
func GenerateOrderID() string {
	rand.Seed(time.Now().UnixNano())
	now := time.Now()
	dateStr := now.Format("20060102150405")
	nanoStr := fmt.Sprintf("%09d", now.Nanosecond())
	randomNum := rand.Intn(10000)
	randomStr := fmt.Sprintf("%04d", randomNum)
	orderID := dateStr + nanoStr + randomStr
	return orderID
}
