// Package nhpcore provides NHP (Network-Hiding Protocol) core functionality for mobile platforms.
// This package is designed to be compiled with gomobile for use in Android and iOS applications.
//
// Build for Android: gomobile bind -target=android -o nhpcore.aar .
// Build for iOS: gomobile bind -target=ios -o Nhpcore.xcframework .
package nhpcore

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/OpenNHP/opennhp/endpoints/agent"
	"github.com/OpenNHP/opennhp/nhp/common"
	_ "golang.org/x/mobile/bind"
)

// Constants
const (
	NhpDomainSuffix    = ".nhp"
	DefaultUpstreamDNS = "8.8.8.8"
	DefaultServerPort  = 62206
	Version            = "1.0.0"
)

// Global NHP agent instance
var (
	gAgentInstance *agent.UdpAgent
	gWorkDir       string
	gLogLevel      int
	gInitMutex     sync.Mutex
	gInitialized   bool

	// Resource mapping: resourceId -> KnockResource
	gResourceMap     map[string]*agent.KnockTarget
	gResourceMapLock sync.RWMutex
)

// InitializeWithConfig initializes the NHP agent with JSON config
// configJSON: JSON string containing agent config, servers, and resources
func InitializeWithConfig(workDir string, logLevel int, configJSON string) error {
	gInitMutex.Lock()
	defer gInitMutex.Unlock()

	if gInitialized {
		return nil
	}

	gWorkDir = workDir
	gLogLevel = logLevel
	gResourceMap = make(map[string]*agent.KnockTarget)

	// Parse config
	var config struct {
		Agent struct {
			PrivateKeyBase64 string `json:"privateKeyBase64"`
			UserId           string `json:"userId"`
			OrganizationId   string `json:"organizationId"`
			CipherScheme     int    `json:"cipherScheme"`
		} `json:"agent"`
		Servers []struct {
			Hostname     string `json:"hostname"`
			Ip           string `json:"ip"`
			Port         int    `json:"port"`
			PubKeyBase64 string `json:"pubKeyBase64"`
			ExpireTime   int64  `json:"expireTime"`
		} `json:"servers"`
		Resources []struct {
			AuthServiceId  string `json:"authServiceId"`
			ResourceId     string `json:"resourceId"`
			ServerIp       string `json:"serverIp"`
			ServerHostname string `json:"serverHostname"`
			ServerPort     int    `json:"serverPort"`
		} `json:"resources"`
	}

	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return err
	}

	// Create agent instance
	gAgentInstance = &agent.UdpAgent{}
	err := gAgentInstance.Start(workDir, logLevel)
	if err != nil {
		gAgentInstance = nil
		return err
	}

	// Add resources
	for _, res := range config.Resources {
		port := res.ServerPort
		if port == 0 {
			port = DefaultServerPort
		}
		resource := agent.KnockResource{
			AuthServiceId:  res.AuthServiceId,
			ResourceId:     res.ResourceId,
			ServerIp:       res.ServerIp,
			ServerHostname: res.ServerHostname,
			ServerPort:     port,
		}

		target := &agent.KnockTarget{
			KnockResource: resource,
			ServerPeer:    gAgentInstance.FindServerPeerFromResource(&resource),
		}

		gResourceMapLock.Lock()
		gResourceMap[res.ResourceId] = target
		gResourceMapLock.Unlock()

	}

	gInitialized = true
	return nil
}

// Cleanup releases all resources
func Cleanup() {
	gInitMutex.Lock()
	defer gInitMutex.Unlock()

	if gAgentInstance != nil {
		gAgentInstance.Stop()
		gAgentInstance = nil
	}

	gResourceMapLock.Lock()
	gResourceMap = nil
	gResourceMapLock.Unlock()

	gInitialized = false
}

// IsNHPDomain checks if the domain ends with .nhp suffix
func IsNHPDomain(domain string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSuffix(domain, ".")), NhpDomainSuffix)
}

// ExtractResourceID extracts the resource ID from an NHP domain
// e.g., "myapp.nhp" -> "myapp", "securitygroup.nhp" -> "securitygroup"
func ExtractResourceID(domain string) string {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if idx := strings.Index(domain, NhpDomainSuffix); idx != -1 {
		return domain[:idx]
	}
	return domain
}

// GetKnockResultJSON performs knock and returns result as JSON
// resourceID: the resource ID extracted from domain (e.g., "demo" from "demo.nhp")
// Returns JSON with fields matching ServerKnockAckMsg: errCode, errMsg, resHost, opnTime, etc.
func GetKnockResultJSON(resourceID string) string {
	// Use same field names as ServerKnockAckMsg for consistency
	result := map[string]interface{}{
		"errCode": "",
		"errMsg":  "",
		"resHost": nil,
		"opnTime": 0,
	}

	if gAgentInstance == nil {
		result["errCode"] = common.ErrNoAgentInstance.ErrorCode()
		result["errMsg"] = "NHP agent not initialized"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	// Find resource by ID
	gResourceMapLock.RLock()
	target, found := gResourceMap[resourceID]
	gResourceMapLock.RUnlock()

	if !found {
		result["errCode"] = "RESOURCE_NOT_FOUND"
		result["errMsg"] = "Unknown resource: " + resourceID
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	ackMsg, err := gAgentInstance.Knock(target)
	if err != nil {
		// Handle case where ackMsg might be nil
		if ackMsg != nil {
			result["errCode"] = ackMsg.ErrCode
			result["errMsg"] = ackMsg.ErrMsg
		}
		if result["errMsg"] == "" || result["errMsg"] == nil {
			result["errMsg"] = err.Error()
		}
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	// Handle case where knock succeeded but ackMsg is nil (shouldn't happen, but be safe)
	if ackMsg == nil {
		result["errCode"] = "UNKNOWN_ERROR"
		result["errMsg"] = "Knock returned no response"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	// Return the full ackMsg which has correct field names
	resultBytes, _ := json.Marshal(ackMsg)
	return string(resultBytes)
}

// IsInitialized returns whether the agent is initialized
func IsInitialized() bool {
	return gInitialized && gAgentInstance != nil
}

// ========== QR Code Authentication Functions ==========

// QRCodeScanResult represents the parsed QR code data
type QRCodeScanResult struct {
	Success   bool   `json:"success"`
	SessionID string `json:"sessionId"`
	Token     string `json:"token"`
	OTPSecret string `json:"otpSecret"`
	AspId     string `json:"aspId"`
	ResId     string `json:"resId"`
	Server    string `json:"server"`
	ErrMsg    string `json:"errMsg,omitempty"`
}

// QRAuthResult represents the result of QR authentication
type QRAuthResult struct {
	Success bool   `json:"success"`
	ErrMsg  string `json:"errMsg,omitempty"`
}

// ParseQRCodeData parses the QR code content and returns structured data
// qrContent: the raw QR code content (e.g., "nhp://scan?data=...&otp=...")
func ParseQRCodeData(qrContent string) string {
	result := QRCodeScanResult{
		Success: false,
	}

	// Parse nhp:// URL scheme
	if !strings.HasPrefix(qrContent, "nhp://scan?") {
		result.ErrMsg = "Invalid QR code format"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	// Parse query parameters
	queryStr := strings.TrimPrefix(qrContent, "nhp://scan?")
	params := make(map[string]string)
	for _, pair := range strings.Split(queryStr, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			// URL decode the value
			decoded, err := urlDecode(kv[1])
			if err != nil {
				params[kv[0]] = kv[1]
			} else {
				params[kv[0]] = decoded
			}
		}
	}

	encryptedData, ok := params["data"]
	if !ok || encryptedData == "" {
		result.ErrMsg = "Missing encrypted data in QR code"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	otpSecret := params["otp"]
	serverUrl := params["server"]
	aspId := params["asp"]
	resId := params["res"]
	sessionId := params["sid"]

	// Store parsed data (encrypted data will be sent to server for verification)
	result.Success = true
	result.SessionID = sessionId // Session ID for scan notification
	result.Token = encryptedData // This is the encrypted QR data
	result.OTPSecret = otpSecret
	result.Server = serverUrl // Server URL for API calls
	result.AspId = aspId      // Auth service provider ID (plugin ID)
	result.ResId = resId      // Resource ID

	resultBytes, _ := json.Marshal(result)
	return string(resultBytes)
}

// urlDecode decodes URL-encoded string
func urlDecode(s string) (string, error) {
	var result strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			var val int
			for j := 0; j < 2; j++ {
				c := s[i+1+j]
				switch {
				case c >= '0' && c <= '9':
					val = val*16 + int(c-'0')
				case c >= 'a' && c <= 'f':
					val = val*16 + int(c-'a'+10)
				case c >= 'A' && c <= 'F':
					val = val*16 + int(c-'A'+10)
				default:
					return "", nil
				}
			}
			result.WriteByte(byte(val))
			i += 3
		} else if s[i] == '+' {
			result.WriteByte(' ')
			i++
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String(), nil
}

// GenerateTOTP generates a TOTP code from the secret
// secret: the base32 encoded TOTP secret
func GenerateTOTP(secret string) string {
	result := map[string]interface{}{
		"success": false,
		"code":    "",
		"errMsg":  "",
	}

	// Use standard TOTP implementation
	code, err := generateTOTPCode(secret)
	if err != nil {
		result["errMsg"] = err.Error()
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	result["success"] = true
	result["code"] = code
	resultBytes, _ := json.Marshal(result)
	return string(resultBytes)
}

// generateTOTPCode generates a 6-digit TOTP code
func generateTOTPCode(secret string) (string, error) {
	// Decode base32 secret
	base32Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	secret = strings.ToUpper(strings.TrimSpace(secret))
	secret = strings.ReplaceAll(secret, "=", "") // Remove padding

	var bits uint64
	var bitCount int
	var decoded []byte

	for _, c := range secret {
		idx := strings.IndexRune(base32Chars, c)
		if idx == -1 {
			continue // Skip invalid characters
		}
		bits = (bits << 5) | uint64(idx)
		bitCount += 5
		if bitCount >= 8 {
			bitCount -= 8
			decoded = append(decoded, byte(bits>>bitCount))
			bits &= (1 << bitCount) - 1
		}
	}

	if len(decoded) == 0 {
		return "", nil
	}

	// Generate TOTP using current time
	counter := uint64(time.Now().Unix() / 30)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	h := hmac.New(sha1.New, decoded)
	h.Write(counterBytes)
	hash := h.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	code = code % 1000000

	return fmt.Sprintf("%06d", code), nil
}

// VerifyQRAuth sends QR authentication verification to the server via GET request
// serverUrl: the NHP server URL (e.g., "https://nhp.example.com")
// encryptedData: the encrypted QR data
// otpCode: the TOTP code generated from the secret
// deviceInfo: device information string
// aspId: auth service provider ID (plugin ID)
// resId: resource ID
func VerifyQRAuth(serverUrl, encryptedData, otpCode, deviceInfo, aspId, resId string) string {
	result := QRAuthResult{
		Success: false,
	}

	// Create secure payload with OTP code included for integrity
	// Format: encryptedData|otpCode|deviceInfo|timestamp
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%s|%s|%s|%d", encryptedData, otpCode, deviceInfo, timestamp)

	// Generate HMAC-SHA256 signature for the payload using OTP code as key
	// This ensures the OTP code cannot be tampered with and provides integrity
	h := hmac.New(sha256.New, []byte(otpCode))
	h.Write([]byte(payload))
	signature := fmt.Sprintf("%x", h.Sum(nil))

	// Make HTTP GET request with query parameters
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Build URL using plugin system format with query parameters
	// Include signature for integrity verification
	baseUrl := strings.TrimSuffix(serverUrl, "/")
	verifyUrl := fmt.Sprintf("%s/plugins/%s?resid=%s&action=verify&encryptedData=%s&otpCode=%s&deviceInfo=%s&ts=%d&sig=%s",
		baseUrl, aspId, resId,
		urlEncode(encryptedData),
		urlEncode(otpCode),
		urlEncode(deviceInfo),
		timestamp,
		signature)

	resp, err := client.Get(verifyUrl)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("Network error: %s (URL: %s)", err.Error(), verifyUrl)
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.ErrMsg = "Failed to read response"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	var serverResp struct {
		Success bool   `json:"success"`
		ErrMsg  string `json:"errMsg"`
	}
	if err := json.Unmarshal(respBody, &serverResp); err != nil {
		result.ErrMsg = fmt.Sprintf("Invalid server response: %s", string(respBody))
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	result.Success = serverResp.Success
	result.ErrMsg = serverResp.ErrMsg
	resBytes, _ := json.Marshal(result)
	return string(resBytes)
}

// NotifyQRScan notifies the server that QR code has been scanned
// This updates the session status from "pending" to "scanned"
// serverUrl: the NHP server URL
// sessionId: the QR session ID (extracted from encrypted data or QR content)
// aspId: auth service provider ID (plugin ID)
// resId: resource ID
func NotifyQRScan(serverUrl, sessionId, aspId, resId string) string {
	result := QRAuthResult{
		Success: false,
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Build URL: /plugins/{aspId}?resid={resId}&action=scan&sessionId=xxx
	baseUrl := strings.TrimSuffix(serverUrl, "/")
	scanUrl := fmt.Sprintf("%s/plugins/%s?resid=%s&action=scan&sessionId=%s",
		baseUrl, aspId, resId, urlEncode(sessionId))

	resp, err := client.Get(scanUrl)
	if err != nil {
		result.ErrMsg = fmt.Sprintf("Network error: %s", err.Error())
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.ErrMsg = "Failed to read response"
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	var serverResp struct {
		Success   bool   `json:"success"`
		ErrMsg    string `json:"errMsg"`
		AspId     string `json:"aspId"`
		ResId     string `json:"resId"`
		OTPSecret string `json:"otpSecret"`
	}
	if err := json.Unmarshal(respBody, &serverResp); err != nil {
		result.ErrMsg = fmt.Sprintf("Invalid server response: %s", string(respBody))
		resultBytes, _ := json.Marshal(result)
		return string(resultBytes)
	}

	result.Success = serverResp.Success
	result.ErrMsg = serverResp.ErrMsg
	resBytes, _ := json.Marshal(result)
	return string(resBytes)
}

// urlEncode encodes a string for use in URL query parameters
func urlEncode(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			result.WriteByte(c)
		} else {
			result.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return result.String()
}
