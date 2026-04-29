package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func main() {
	apiKey := "eyJvcmciOiJkbnNlIiwiaWQiOiI3YTdmYjRiNWM0ZGM0ODhiOWQ0ZmJmOWZmZjE0YTllMiIsImgiOiJtdXJtdXIxMjgifQ=="
	apiSecret := "j5x4a-oTlfa6NtWdoclIp6wi-xLC2cN-0pWfU0I3tdJUKvMC_DuAsQlANmyZO3eyGKPcX2UK-fg29KION1m8Bg"

	method := "get"
	path := "/accounts"
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 +0000")

	var nonceBytes [16]byte
	rand.Read(nonceBytes[:])
	nonce := hex.EncodeToString(nonceBytes[:])

	signingString := fmt.Sprintf("(request-target): %s %s\ndate: %s\nnonce: %s", strings.ToLower(method), path, date, nonce)

	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(signingString))
	raw := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	escaped := url.QueryEscape(raw)

	sigHeader := fmt.Sprintf(
		`Signature keyId="%s",algorithm="hmac-sha256",headers="(request-target) date",signature="%s",nonce="%s"`,
		apiKey, escaped, nonce,
	)

	fmt.Printf("date:        %s\nnonce:       %s\nsig raw b64: %s\nx-signature: %s\n\n", date, nonce, raw, sigHeader)

	req, _ := http.NewRequest("GET", "https://openapi.dnse.com.vn/accounts", nil)
	req.Header["x-api-key"] = []string{apiKey}
	req.Header["date"] = []string{date}
	req.Header["x-signature"] = []string{sigHeader}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
}
