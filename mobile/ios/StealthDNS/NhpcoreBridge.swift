import Foundation
import Nhpcore  // Import the Go library

// MARK: - NHP Core Bridge
// This file bridges the Go nhpcore library to Swift

/// Initialize the NHP core library with JSON config
func NhpcoreInitializeWithConfig(_ workDir: String, _ logLevel: Int, _ configJSON: String) throws {
    var error: NSError?
    NhpcoreInitializeWithConfig(workDir, Int(logLevel), configJSON, &error)
    if let error = error {
        throw error
    }
}

/// Check if a domain is an NHP domain
func NhpcoreIsNHPDomain(_ domain: String) -> Bool {
    return Nhpcore.NhpcoreIsNHPDomain(domain)
}

/// Extract resource ID from domain
func NhpcoreExtractResourceID(_ domain: String) -> String {
    return Nhpcore.NhpcoreExtractResourceID(domain)
}

/// Perform NHP knock and get result as JSON
func NhpcoreGetKnockResultJSON(_ resourceId: String) -> String {
    return Nhpcore.NhpcoreGetKnockResultJSON(resourceId)
}

/// Check if NHP is initialized
func NhpcoreIsInitialized() -> Bool {
    return Nhpcore.NhpcoreIsInitialized()
}

/// Cleanup NHP resources
func NhpcoreCleanup() {
    Nhpcore.NhpcoreCleanup()
}

// MARK: - QR Code Authentication Functions

/// Parse QR code data and return structured JSON
func NhpcoreParseQRCodeData(_ qrContent: String) -> String {
    return Nhpcore.NhpcoreParseQRCodeData(qrContent)
}

/// Generate TOTP code from secret
func NhpcoreGenerateTOTP(_ secret: String) -> String {
    return Nhpcore.NhpcoreGenerateTOTP(secret)
}

/// Verify QR authentication with server
func NhpcoreVerifyQRAuth(_ serverUrl: String, _ encryptedData: String, _ otpCode: String, _ deviceInfo: String, _ aspId: String, _ resId: String) -> String {
    return Nhpcore.NhpcoreVerifyQRAuth(serverUrl, encryptedData, otpCode, deviceInfo, aspId, resId)
}

/// Notify server that QR code was scanned
func NhpcoreNotifyQRScan(_ serverUrl: String, _ sessionId: String, _ aspId: String, _ resId: String) -> String {
    return Nhpcore.NhpcoreNotifyQRScan(serverUrl, sessionId, aspId, resId)
}
