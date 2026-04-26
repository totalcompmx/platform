package cookies

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/internal/assert"
)

func TestWriteAndRead(t *testing.T) {
	t.Run("Round trip encodes and decodes cookie value", func(t *testing.T) {
		w := httptest.NewRecorder()

		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value with special chars!便#ي%",
		}

		err := Write(w, cookie)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		value, err := Read(req, "test_cookie")
		assert.Nil(t, err)
		assert.Equal(t, "this is a test value with special chars!便#ي%", value)
	})

	t.Run("Returns ErrValueTooLong when cookie exceeds 4096 bytes", func(t *testing.T) {
		w := httptest.NewRecorder()

		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: strings.Repeat("a", 4000),
		}

		err := Write(w, cookie)
		assert.Equal(t, err, ErrValueTooLong)
	})

	t.Run("Returns ErrInvalidValue for tampered base64", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "test", Value: "invalid-base64!"})

		_, err := Read(req, "test")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns error when cookie is missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		_, err := Read(req, "missing")
		assert.NotNil(t, err)
	})
}

func TestWriteSignedAndReadSigned(t *testing.T) {
	t.Run("Round trip signs and verifies cookie value", func(t *testing.T) {
		w := httptest.NewRecorder()

		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		secretKey := "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o"
		err := WriteSigned(w, cookie, secretKey)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		t.Log(cookies[0].Value)

		value, err := ReadSigned(req, "test_cookie", secretKey)
		assert.Nil(t, err)
		assert.Equal(t, "this is a test value", value)
	})

	t.Run("Returns ErrInvalidValue with wrong secret key", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		err := WriteSigned(w, cookie, "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.Nil(t, err)

		cookies := w.Result().Cookies()
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		_, err = ReadSigned(req, "test_cookie", "wrongSecretKeyAX7v2WqLpJ3nZcRYKt")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns ErrInvalidValue for tampered signature", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		secretKey := "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o"
		err := WriteSigned(w, cookie, secretKey)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()

		if cookies[0].Value[0] == 'a' {
			cookies[0].Value = "b" + cookies[0].Value[1:]
		} else {
			cookies[0].Value = "a" + cookies[0].Value[1:]
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		_, err = ReadSigned(req, "test_cookie", secretKey)
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns ErrInvalidValue for value shorter than signature", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		req.AddCookie(&http.Cookie{Name: "test_cookie", Value: "dGVzdA=="})

		_, err := ReadSigned(req, "test_cookie", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns error when signed cookie is missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		_, err := ReadSigned(req, "missing", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.NotNil(t, err)
	})
}

func TestWriteEncryptedAndReadEncrypted(t *testing.T) {
	t.Run("Round trip encrypts and decrypts cookie value", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		secretKey := "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o"
		err := WriteEncrypted(w, cookie, secretKey)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()

		decodedValue, err := base64.URLEncoding.DecodeString(cookies[0].Value)
		assert.Nil(t, err)
		assert.False(t, strings.Contains(string(decodedValue), "this is a test value"))

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		value, err := ReadEncrypted(req, "test_cookie", secretKey)
		assert.Nil(t, err)
		assert.Equal(t, "this is a test value", value)
	})

	t.Run("Returns ErrInvalidValue with wrong decryption key", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		secretKey := "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o"
		err := WriteEncrypted(w, cookie, secretKey)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		_, err = ReadEncrypted(req, "test_cookie", "wrongSecretKeyAX7v2WqLpJ3nZcRYKt")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns ErrInvalidValue for tampered encrypted data", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{
			Name:  "test_cookie",
			Value: "this is a test value",
		}

		secretKey := "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o"
		err := WriteEncrypted(w, cookie, secretKey)
		assert.Nil(t, err)

		cookies := w.Result().Cookies()

		if cookies[0].Value[0] == 'a' {
			cookies[0].Value = "b" + cookies[0].Value[1:]
		} else {
			cookies[0].Value = "a" + cookies[0].Value[1:]
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(cookies[0])

		_, err = ReadEncrypted(req, "test_cookie", secretKey)
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns ErrInvalidValue for value shorter than nonce", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		req.AddCookie(&http.Cookie{Name: "test", Value: "dGVzdA=="})

		_, err := ReadEncrypted(req, "test", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns error when encrypted cookie is missing", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)

		_, err := ReadEncrypted(req, "missing", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.NotNil(t, err)
	})

	t.Run("Returns error for invalid encryption key", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{Name: "test", Value: "value"}

		err := WriteEncrypted(w, cookie, "short-key")
		assert.NotNil(t, err)
	})

	t.Run("Returns error when nonce generation fails", func(t *testing.T) {
		w := httptest.NewRecorder()
		cookie := http.Cookie{Name: "test", Value: "value"}

		oldReader := randomReader
		randomReader = errReader{}
		defer func() { randomReader = oldReader }()

		err := WriteEncrypted(w, cookie, "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.NotNil(t, err)
	})

	t.Run("Returns error when encrypted cookie key is invalid", func(t *testing.T) {
		req := encryptedCookieRequest(t, "real_name", "value", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		cookie := req.Cookies()[0]
		cookie.Name = "wrong_name"
		req.Header.Set("Cookie", cookie.String())

		_, err := ReadEncrypted(req, "wrong_name", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")
		assert.Equal(t, err, ErrInvalidValue)
	})

	t.Run("Returns error when encrypted cookie key cannot be created", func(t *testing.T) {
		req := encryptedCookieRequest(t, "test", "value", "mySecretKeyAX7v2WqLpJ3nZcRYKtM9o")

		_, err := ReadEncrypted(req, "test", "short-key")
		assert.NotNil(t, err)
	})
}

func TestEncryptedCookieValue(t *testing.T) {
	t.Run("Returns ErrInvalidValue when plaintext has no separator", func(t *testing.T) {
		_, err := encryptedCookieValue("missing-separator", "test")
		assert.Equal(t, err, ErrInvalidValue)
	})
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

func encryptedCookieRequest(t *testing.T, name string, value string, secretKey string) *http.Request {
	t.Helper()

	w := httptest.NewRecorder()
	err := WriteEncrypted(w, http.Cookie{Name: name, Value: value}, secretKey)
	assert.Nil(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(w.Result().Cookies()[0])
	return req
}

var _ io.Reader = errReader{}
