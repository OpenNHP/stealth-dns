import UIKit
import AVFoundation
import AudioToolbox

/**
 * QR Code Scanner View Controller for NHP authentication
 */
class QRScanViewController: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
    
    // MARK: - Properties
    private var captureSession: AVCaptureSession?
    private var previewLayer: AVCaptureVideoPreviewLayer?
    private var isProcessing = false
    private var scannedQRData: String?
    
    // MARK: - UI Components
    private let closeButton = UIButton(type: .system)
    private let flashButton = UIButton(type: .system)
    private let titleLabel = UILabel()
    private let statusLabel = UILabel()
    private let instructionLabel = UILabel()
    private let scanFrameView = UIView()
    private let scanLineView = UIView()
    private var scanLineAnimator: UIViewPropertyAnimator?
    private let resultContainerView = UIView()
    private let resultTitleLabel = UILabel()
    private let resultMessageLabel = UILabel()
    private let confirmButton = UIButton(type: .system)
    private let cancelButton = UIButton(type: .system)
    private let activityIndicator = UIActivityIndicatorView(style: .large)
    
    // MARK: - Colors
    private let backgroundColor = UIColor(red: 0.043, green: 0.063, blue: 0.125, alpha: 1.0)
    private let nhpBlue = UIColor(red: 0.161, green: 0.714, blue: 0.965, alpha: 1.0)
    private let nhpGreen = UIColor(red: 0.298, green: 0.686, blue: 0.314, alpha: 1.0)
    private let textPrimary = UIColor.white
    private let textSecondary = UIColor(white: 0.7, alpha: 1.0)
    
    private var isFlashOn = false
    
    // MARK: - Lifecycle
    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
        checkCameraPermission()
    }
    
    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        startScanLineAnimation()
    }
    
    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        stopScanLineAnimation()
        stopScanning()
    }
    
    override var prefersStatusBarHidden: Bool {
        return true
    }
    
    // MARK: - UI Setup
    private func setupUI() {
        view.backgroundColor = backgroundColor
        
        // Close button
        closeButton.setImage(UIImage(systemName: "xmark"), for: .normal)
        closeButton.tintColor = .white
        closeButton.translatesAutoresizingMaskIntoConstraints = false
        closeButton.addTarget(self, action: #selector(closeTapped), for: .touchUpInside)
        view.addSubview(closeButton)
        
        // Flash button
        flashButton.setImage(UIImage(systemName: "bolt.slash"), for: .normal)
        flashButton.tintColor = .white
        flashButton.translatesAutoresizingMaskIntoConstraints = false
        flashButton.addTarget(self, action: #selector(flashTapped), for: .touchUpInside)
        view.addSubview(flashButton)
        
        // Title label
        titleLabel.text = "Scan QR Code"
        titleLabel.textColor = textPrimary
        titleLabel.font = UIFont.boldSystemFont(ofSize: 18)
        titleLabel.textAlignment = .center
        titleLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(titleLabel)
        
        // Scan frame
        scanFrameView.layer.borderColor = nhpBlue.cgColor
        scanFrameView.layer.borderWidth = 3
        scanFrameView.layer.cornerRadius = 16
        scanFrameView.backgroundColor = .clear
        scanFrameView.clipsToBounds = true
        scanFrameView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(scanFrameView)
        
        // Scan line (animated laser effect)
        scanLineView.translatesAutoresizingMaskIntoConstraints = false
        scanFrameView.addSubview(scanLineView)
        
        // Create gradient layer for scan line
        let gradientLayer = CAGradientLayer()
        gradientLayer.colors = [
            UIColor.clear.cgColor,
            nhpBlue.cgColor,
            UIColor.clear.cgColor
        ]
        gradientLayer.startPoint = CGPoint(x: 0, y: 0.5)
        gradientLayer.endPoint = CGPoint(x: 1, y: 0.5)
        gradientLayer.frame = CGRect(x: 0, y: 0, width: 234, height: 3)
        scanLineView.layer.addSublayer(gradientLayer)
        
        // Instruction label
        instructionLabel.text = "Point camera at NHP login QR code"
        instructionLabel.textColor = textPrimary
        instructionLabel.font = UIFont.systemFont(ofSize: 16)
        instructionLabel.textAlignment = .center
        instructionLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(instructionLabel)
        
        // Status label
        statusLabel.text = "Scanning..."
        statusLabel.textColor = nhpBlue
        statusLabel.font = UIFont.systemFont(ofSize: 14)
        statusLabel.textAlignment = .center
        statusLabel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(statusLabel)
        
        // Activity indicator
        activityIndicator.color = nhpBlue
        activityIndicator.hidesWhenStopped = true
        activityIndicator.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(activityIndicator)
        
        // Result container
        resultContainerView.backgroundColor = UIColor(red: 0.1, green: 0.16, blue: 0.29, alpha: 1.0)
        resultContainerView.layer.cornerRadius = 16
        resultContainerView.layer.borderWidth = 1
        resultContainerView.layer.borderColor = nhpBlue.withAlphaComponent(0.3).cgColor
        resultContainerView.isHidden = true
        resultContainerView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(resultContainerView)
        
        // Result title
        resultTitleLabel.text = "NHP Login Request"
        resultTitleLabel.textColor = textPrimary
        resultTitleLabel.font = UIFont.boldSystemFont(ofSize: 20)
        resultTitleLabel.textAlignment = .center
        resultTitleLabel.translatesAutoresizingMaskIntoConstraints = false
        resultContainerView.addSubview(resultTitleLabel)
        
        // Result message
        resultMessageLabel.text = "Confirm to authenticate and login?"
        resultMessageLabel.textColor = textSecondary
        resultMessageLabel.font = UIFont.systemFont(ofSize: 14)
        resultMessageLabel.textAlignment = .center
        resultMessageLabel.numberOfLines = 0
        resultMessageLabel.translatesAutoresizingMaskIntoConstraints = false
        resultContainerView.addSubview(resultMessageLabel)
        
        // Confirm button
        confirmButton.setTitle("Confirm", for: .normal)
        confirmButton.setTitleColor(backgroundColor, for: .normal)
        confirmButton.backgroundColor = nhpBlue
        confirmButton.layer.cornerRadius = 8
        confirmButton.titleLabel?.font = UIFont.boldSystemFont(ofSize: 16)
        confirmButton.translatesAutoresizingMaskIntoConstraints = false
        confirmButton.addTarget(self, action: #selector(confirmTapped), for: .touchUpInside)
        resultContainerView.addSubview(confirmButton)
        
        // Cancel button
        cancelButton.setTitle("Cancel", for: .normal)
        cancelButton.setTitleColor(textPrimary, for: .normal)
        cancelButton.backgroundColor = UIColor(red: 0.1, green: 0.16, blue: 0.29, alpha: 1.0)
        cancelButton.layer.cornerRadius = 8
        cancelButton.layer.borderWidth = 1
        cancelButton.layer.borderColor = UIColor.gray.cgColor
        cancelButton.titleLabel?.font = UIFont.systemFont(ofSize: 16)
        cancelButton.translatesAutoresizingMaskIntoConstraints = false
        cancelButton.addTarget(self, action: #selector(cancelTapped), for: .touchUpInside)
        resultContainerView.addSubview(cancelButton)
        
        // Constraints
        NSLayoutConstraint.activate([
            closeButton.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 16),
            closeButton.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 16),
            closeButton.widthAnchor.constraint(equalToConstant: 44),
            closeButton.heightAnchor.constraint(equalToConstant: 44),
            
            flashButton.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 16),
            flashButton.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -16),
            flashButton.widthAnchor.constraint(equalToConstant: 44),
            flashButton.heightAnchor.constraint(equalToConstant: 44),
            
            titleLabel.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 24),
            titleLabel.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            
            scanFrameView.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            scanFrameView.centerYAnchor.constraint(equalTo: view.centerYAnchor, constant: -40),
            scanFrameView.widthAnchor.constraint(equalToConstant: 250),
            scanFrameView.heightAnchor.constraint(equalToConstant: 250),
            
            scanLineView.leadingAnchor.constraint(equalTo: scanFrameView.leadingAnchor, constant: 8),
            scanLineView.trailingAnchor.constraint(equalTo: scanFrameView.trailingAnchor, constant: -8),
            scanLineView.topAnchor.constraint(equalTo: scanFrameView.topAnchor, constant: 8),
            scanLineView.heightAnchor.constraint(equalToConstant: 3),
            
            instructionLabel.topAnchor.constraint(equalTo: scanFrameView.bottomAnchor, constant: 32),
            instructionLabel.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 24),
            instructionLabel.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -24),
            
            statusLabel.topAnchor.constraint(equalTo: instructionLabel.bottomAnchor, constant: 8),
            statusLabel.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            
            activityIndicator.topAnchor.constraint(equalTo: statusLabel.bottomAnchor, constant: 16),
            activityIndicator.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            
            resultContainerView.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            resultContainerView.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            resultContainerView.widthAnchor.constraint(equalTo: view.widthAnchor, constant: -64),
            
            resultTitleLabel.topAnchor.constraint(equalTo: resultContainerView.topAnchor, constant: 24),
            resultTitleLabel.leadingAnchor.constraint(equalTo: resultContainerView.leadingAnchor, constant: 16),
            resultTitleLabel.trailingAnchor.constraint(equalTo: resultContainerView.trailingAnchor, constant: -16),
            
            resultMessageLabel.topAnchor.constraint(equalTo: resultTitleLabel.bottomAnchor, constant: 16),
            resultMessageLabel.leadingAnchor.constraint(equalTo: resultContainerView.leadingAnchor, constant: 16),
            resultMessageLabel.trailingAnchor.constraint(equalTo: resultContainerView.trailingAnchor, constant: -16),
            
            cancelButton.topAnchor.constraint(equalTo: resultMessageLabel.bottomAnchor, constant: 24),
            cancelButton.leadingAnchor.constraint(equalTo: resultContainerView.leadingAnchor, constant: 16),
            cancelButton.heightAnchor.constraint(equalToConstant: 48),
            cancelButton.widthAnchor.constraint(equalTo: resultContainerView.widthAnchor, multiplier: 0.5, constant: -24),
            cancelButton.bottomAnchor.constraint(equalTo: resultContainerView.bottomAnchor, constant: -24),
            
            confirmButton.topAnchor.constraint(equalTo: resultMessageLabel.bottomAnchor, constant: 24),
            confirmButton.trailingAnchor.constraint(equalTo: resultContainerView.trailingAnchor, constant: -16),
            confirmButton.heightAnchor.constraint(equalToConstant: 48),
            confirmButton.widthAnchor.constraint(equalTo: resultContainerView.widthAnchor, multiplier: 0.5, constant: -24),
        ])
    }
    
    // MARK: - Camera Setup
    private func checkCameraPermission() {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            setupCamera()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { [weak self] granted in
                DispatchQueue.main.async {
                    if granted {
                        self?.setupCamera()
                    } else {
                        self?.showPermissionDenied()
                    }
                }
            }
        default:
            showPermissionDenied()
        }
    }
    
    private func setupCamera() {
        captureSession = AVCaptureSession()
        
        guard let captureSession = captureSession else { return }
        
        // Set high resolution for better QR detection
        if captureSession.canSetSessionPreset(.hd1280x720) {
            captureSession.sessionPreset = .hd1280x720
        } else if captureSession.canSetSessionPreset(.high) {
            captureSession.sessionPreset = .high
        }
        
        guard let videoCaptureDevice = AVCaptureDevice.default(.builtInWideAngleCamera, for: .video, position: .back) else {
            showError("Unable to access camera")
            return
        }
        
        // Configure camera for fast autofocus
        do {
            try videoCaptureDevice.lockForConfiguration()
            
            // Enable continuous autofocus for faster scanning
            if videoCaptureDevice.isFocusModeSupported(.continuousAutoFocus) {
                videoCaptureDevice.focusMode = .continuousAutoFocus
            }
            
            // Enable auto exposure
            if videoCaptureDevice.isExposureModeSupported(.continuousAutoExposure) {
                videoCaptureDevice.exposureMode = .continuousAutoExposure
            }
            
            // Set focus point to center (where QR code typically is)
            if videoCaptureDevice.isFocusPointOfInterestSupported {
                videoCaptureDevice.focusPointOfInterest = CGPoint(x: 0.5, y: 0.5)
            }
            
            videoCaptureDevice.unlockForConfiguration()
        } catch {
            print("Camera configuration error: \(error)")
        }
        
        let videoInput: AVCaptureDeviceInput
        do {
            videoInput = try AVCaptureDeviceInput(device: videoCaptureDevice)
        } catch {
            showError("Unable to create video input: \(error.localizedDescription)")
            return
        }
        
        guard captureSession.canAddInput(videoInput) else {
            showError("Unable to add video input")
            return
        }
        captureSession.addInput(videoInput)
        
        let metadataOutput = AVCaptureMetadataOutput()
        guard captureSession.canAddOutput(metadataOutput) else {
            showError("Unable to add metadata output")
            return
        }
        captureSession.addOutput(metadataOutput)
        
        // Set delegate with high priority queue for faster processing
        metadataOutput.setMetadataObjectsDelegate(self, queue: DispatchQueue.main)
        metadataOutput.metadataObjectTypes = [.qr]
        
        previewLayer = AVCaptureVideoPreviewLayer(session: captureSession)
        previewLayer?.frame = view.layer.bounds
        previewLayer?.videoGravity = .resizeAspectFill
        
        if let previewLayer = previewLayer {
            view.layer.insertSublayer(previewLayer, at: 0)
        }
        
        // Start camera on background thread
        DispatchQueue.global(qos: .userInteractive).async { [weak self] in
            self?.captureSession?.startRunning()
            
            // Set rect of interest after session starts (for faster scanning in center area)
            DispatchQueue.main.async {
                self?.updateScanRectOfInterest(metadataOutput)
            }
        }
    }
    
    private func updateScanRectOfInterest(_ metadataOutput: AVCaptureMetadataOutput) {
        guard let previewLayer = previewLayer else { return }
        
        // Calculate the scan frame rect in preview layer coordinates
        let scanFrameRect = scanFrameView.frame
        let rectOfInterest = previewLayer.metadataOutputRectConverted(fromLayerRect: scanFrameRect)
        
        // Expand the rect slightly for better detection at edges
        let expandedRect = CGRect(
            x: max(0, rectOfInterest.origin.x - 0.05),
            y: max(0, rectOfInterest.origin.y - 0.05),
            width: min(1, rectOfInterest.width + 0.1),
            height: min(1, rectOfInterest.height + 0.1)
        )
        
        metadataOutput.rectOfInterest = expandedRect
    }
    
    private func showPermissionDenied() {
        let alert = UIAlertController(
            title: "Camera Permission Required",
            message: "Please enable camera access in Settings to scan QR codes.",
            preferredStyle: .alert
        )
        alert.addAction(UIAlertAction(title: "Settings", style: .default) { _ in
            if let settingsUrl = URL(string: UIApplication.openSettingsURLString) {
                UIApplication.shared.open(settingsUrl)
            }
        })
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel) { [weak self] _ in
            self?.dismiss(animated: true)
        })
        present(alert, animated: true)
    }
    
    private func stopScanning() {
        captureSession?.stopRunning()
    }
    
    // MARK: - Scan Line Animation
    private func startScanLineAnimation() {
        // Reset scan line position
        scanLineView.transform = .identity
        
        // Animation distance (250dp frame - 16dp margins - 3dp line height)
        let animationDistance: CGFloat = 250 - 16 - 3
        
        animateScanLine(distance: animationDistance)
    }
    
    private func animateScanLine(distance: CGFloat) {
        // Animate down
        UIView.animate(withDuration: 2.0, delay: 0, options: [.curveLinear]) { [weak self] in
            self?.scanLineView.transform = CGAffineTransform(translationX: 0, y: distance)
        } completion: { [weak self] finished in
            guard finished else { return }
            // Animate back up
            UIView.animate(withDuration: 2.0, delay: 0, options: [.curveLinear]) { [weak self] in
                self?.scanLineView.transform = .identity
            } completion: { [weak self] finished in
                guard finished else { return }
                // Repeat animation
                self?.animateScanLine(distance: distance)
            }
        }
    }
    
    private func stopScanLineAnimation() {
        scanLineView.layer.removeAllAnimations()
    }
    
    // MARK: - AVCaptureMetadataOutputObjectsDelegate
    func metadataOutput(_ output: AVCaptureMetadataOutput, didOutput metadataObjects: [AVMetadataObject], from connection: AVCaptureConnection) {
        guard !isProcessing,
              let metadataObject = metadataObjects.first,
              let readableObject = metadataObject as? AVMetadataMachineReadableCodeObject,
              let stringValue = readableObject.stringValue else {
            return
        }
        
        handleQRCodeScanned(stringValue)
    }
    
    // MARK: - QR Code Handling
    private func handleQRCodeScanned(_ qrData: String) {
        guard !isProcessing else { return }
        isProcessing = true
        
        // Validate NHP QR code
        guard qrData.hasPrefix("nhp://scan?") else {
            statusLabel.text = "Invalid QR code. Please scan an NHP login QR."
            isProcessing = false
            return
        }
        
        scannedQRData = qrData
        AudioServicesPlaySystemSound(SystemSoundID(kSystemSoundID_Vibrate))
        
        DispatchQueue.main.async { [weak self] in
            self?.statusLabel.text = "Notifying server..."
        }
        
        // Parse QR data and notify server
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let parseResult = NhpcoreParseQRCodeData(qrData)
            
            guard let resultData = parseResult.data(using: .utf8),
                  let result = try? JSONSerialization.jsonObject(with: resultData) as? [String: Any] else {
                DispatchQueue.main.async {
                    self?.statusLabel.text = "Failed to parse QR code"
                    self?.isProcessing = false
                }
                return
            }
            
            guard result["success"] as? Bool == true else {
                DispatchQueue.main.async {
                    let errMsg = result["errMsg"] as? String ?? "Failed to parse QR code"
                    self?.statusLabel.text = errMsg
                    self?.isProcessing = false
                }
                return
            }
            
            // Notify server that QR code has been scanned
            let serverUrl = result["server"] as? String ?? ""
            let sessionId = result["sessionId"] as? String ?? ""
            let aspId = result["aspId"] as? String ?? "example"
            let resId = result["resId"] as? String ?? "demo"
            
            if !serverUrl.isEmpty && !sessionId.isEmpty {
                print("Notifying server of QR scan: sessionId=\(sessionId)")
                
                let scanResultStr = NhpcoreNotifyQRScan(serverUrl, sessionId, aspId, resId)
                if let scanData = scanResultStr.data(using: .utf8),
                   let scanResult = try? JSONSerialization.jsonObject(with: scanData) as? [String: Any] {
                    if scanResult["success"] as? Bool == true {
                        print("Server notified successfully")
                    } else {
                        print("Scan notification failed: \(scanResult["errMsg"] ?? "unknown")")
                    }
                }
            }
            
            DispatchQueue.main.async {
                self?.showConfirmDialog(result)
            }
        }
    }
    
    private func showConfirmDialog(_ parseResult: [String: Any]) {
        previewLayer?.opacity = 0.3
        resultContainerView.isHidden = false
        
        let serverUrl = parseResult["server"] as? String ?? ""
        let sessionId = parseResult["sessionId"] as? String ?? ""
        let aspId = parseResult["aspId"] as? String ?? "example"
        let resId = parseResult["resId"] as? String ?? "demo"
        
        statusLabel.text = "Scanned successfully"
        resultTitleLabel.text = "NHP Login"
        resultMessageLabel.text = "Confirm to login?"
        
        // Debug log (only in console)
        print("=== QR Code Parsed ===")
        print("Server URL: \(serverUrl)")
        print("Session ID: \(sessionId)")
        print("AspId: \(aspId), ResId: \(resId)")
        print("======================")
    }
    
    // MARK: - Actions
    @objc private func closeTapped() {
        dismiss(animated: true)
    }
    
    @objc private func flashTapped() {
        guard let device = AVCaptureDevice.default(for: .video), device.hasTorch else { return }
        
        do {
            try device.lockForConfiguration()
            isFlashOn.toggle()
            device.torchMode = isFlashOn ? .on : .off
            flashButton.setImage(UIImage(systemName: isFlashOn ? "bolt.fill" : "bolt.slash"), for: .normal)
            flashButton.tintColor = isFlashOn ? .yellow : .white
            device.unlockForConfiguration()
        } catch {
            print("Flash error: \(error)")
        }
    }
    
    @objc private func confirmTapped() {
        guard let qrData = scannedQRData else { return }
        
        activityIndicator.startAnimating()
        confirmButton.isEnabled = false
        cancelButton.isEnabled = false
        statusLabel.text = "Authenticating..."
        
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.verifyQRAuth(qrData)
        }
    }
    
    @objc private func cancelTapped() {
        resetScanner()
    }
    
    private func verifyQRAuth(_ qrData: String) {
        // Parse QR data
        let parseResultStr = NhpcoreParseQRCodeData(qrData)
        guard let parseData = parseResultStr.data(using: .utf8),
              let parseResult = try? JSONSerialization.jsonObject(with: parseData) as? [String: Any],
              parseResult["success"] as? Bool == true else {
            DispatchQueue.main.async { [weak self] in
                self?.showAuthError("Failed to parse QR code")
            }
            return
        }
        
        let encryptedData = parseResult["token"] as? String ?? ""
        let otpSecret = parseResult["otpSecret"] as? String ?? ""
        
        // Generate TOTP
        let totpResultStr = NhpcoreGenerateTOTP(otpSecret)
        guard let totpData = totpResultStr.data(using: .utf8),
              let totpResult = try? JSONSerialization.jsonObject(with: totpData) as? [String: Any],
              totpResult["success"] as? Bool == true else {
            DispatchQueue.main.async { [weak self] in
                self?.showAuthError("Failed to generate OTP code")
            }
            return
        }
        
        let otpCode = totpResult["code"] as? String ?? ""
        
        // Get device info
        let deviceInfo = "\(UIDevice.current.model) \(UIDevice.current.systemVersion)"
        
        // Get server URL from parsed QR data (now embedded in QR code)
        var serverUrl = parseResult["server"] as? String ?? ""
        if serverUrl.isEmpty {
            // Fallback to extracting from raw QR data
            serverUrl = extractServerUrl(from: qrData)
        }
        
        // Get aspId and resId from parsed QR data
        let aspId = parseResult["aspId"] as? String ?? "example"
        let resId = parseResult["resId"] as? String ?? "demo"
        
        // Build request URL using plugin system format
        let requestUrl = "\(serverUrl)/plugins/\(aspId)?resid=\(resId)&action=verify"
        
        print("=== QR Auth Debug Info ===")
        print("Server URL (from QR): \(serverUrl)")
        print("Plugin ID (aspId): \(aspId)")
        print("Resource ID (resId): \(resId)")
        print("Request URL: \(requestUrl)")
        print("OTP Code: \(otpCode)")
        print("==========================")
        
        // Verify with server using plugin system format
        let verifyResultStr = NhpcoreVerifyQRAuth(serverUrl, encryptedData, otpCode, deviceInfo, aspId, resId)
        guard let verifyData = verifyResultStr.data(using: .utf8),
              let verifyResult = try? JSONSerialization.jsonObject(with: verifyData) as? [String: Any] else {
            DispatchQueue.main.async { [weak self] in
                self?.showAuthError("Invalid server response")
            }
            return
        }
        
        DispatchQueue.main.async { [weak self] in
            self?.activityIndicator.stopAnimating()
            
            if verifyResult["success"] as? Bool == true {
                self?.showAuthSuccess()
            } else {
                let errMsg = verifyResult["errMsg"] as? String ?? "Authentication failed"
                self?.showAuthError(errMsg)
            }
        }
    }
    
    private func extractServerUrl(from qrData: String) -> String {
        // Parse QR data to extract server URL
        let queryStr = qrData.replacingOccurrences(of: "nhp://scan?", with: "")
        var params: [String: String] = [:]
        
        for pair in queryStr.split(separator: "&") {
            let kv = pair.split(separator: "=", maxSplits: 1)
            if kv.count == 2 {
                let key = String(kv[0])
                let value = String(kv[1]).removingPercentEncoding ?? String(kv[1])
                params[key] = value
            }
        }
        
        return params["server"] ?? "https://nhp.opennhp.org"
    }
    
    private func showAuthSuccess() {
        resultTitleLabel.text = "✓ Login Successful"
        resultMessageLabel.text = "You are now logged in."
        statusLabel.text = "Success!"
        statusLabel.textColor = nhpGreen
        
        confirmButton.setTitle("Done", for: .normal)
        confirmButton.isEnabled = true
        confirmButton.removeTarget(self, action: #selector(confirmTapped), for: .touchUpInside)
        confirmButton.addTarget(self, action: #selector(closeTapped), for: .touchUpInside)
        cancelButton.isHidden = true
    }
    
    private func showAuthError(_ message: String) {
        resultTitleLabel.text = "✗ Login Failed"
        resultMessageLabel.text = "Error: \(message)"
        statusLabel.text = "Failed"
        statusLabel.textColor = .red
        
        confirmButton.isEnabled = true
        cancelButton.isEnabled = true
    }
    
    private func resetScanner() {
        isProcessing = false
        scannedQRData = nil
        resultContainerView.isHidden = true
        previewLayer?.opacity = 1.0
        statusLabel.text = "Scanning..."
        statusLabel.textColor = nhpBlue
        confirmButton.setTitle("Confirm", for: .normal)
        confirmButton.isEnabled = true
        confirmButton.removeTarget(self, action: #selector(closeTapped), for: .touchUpInside)
        confirmButton.addTarget(self, action: #selector(confirmTapped), for: .touchUpInside)
        cancelButton.isHidden = false
        cancelButton.isEnabled = true
    }
    
    private func showError(_ message: String) {
        let alert = UIAlertController(title: "Error", message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "OK", style: .default) { [weak self] _ in
            self?.dismiss(animated: true)
        })
        present(alert, animated: true)
    }
}

