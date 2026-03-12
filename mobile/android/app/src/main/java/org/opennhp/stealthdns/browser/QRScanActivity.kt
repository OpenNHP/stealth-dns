package org.opennhp.stealthdns.browser

import android.Manifest
import android.animation.ObjectAnimator
import android.animation.ValueAnimator
import android.content.pm.PackageManager
import android.os.Bundle
import android.util.Log
import android.util.Size
import android.view.View
import android.view.animation.LinearInterpolator
import android.widget.*
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.*
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScanner
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/**
 * QR Code Scanner Activity for NHP authentication
 */
class QRScanActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "QRScanActivity"
        private const val CAMERA_PERMISSION_CODE = 1001
    }

    private lateinit var previewView: PreviewView
    private lateinit var statusText: TextView
    private lateinit var instructionText: TextView
    private lateinit var closeButton: ImageButton
    private lateinit var flashButton: ImageButton
    private lateinit var progressBar: ProgressBar
    private lateinit var resultContainer: LinearLayout
    private lateinit var resultTitle: TextView
    private lateinit var resultMessage: TextView
    private lateinit var confirmButton: Button
    private lateinit var cancelButton: Button
    private lateinit var scanLine: View

    private var cameraExecutor: ExecutorService? = null
    private var camera: Camera? = null
    private var scanLineAnimator: ObjectAnimator? = null
    private var isFlashOn = false
    private var isProcessing = false
    private var scannedQRData: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_qr_scan)
        
        initViews()
        setupListeners()
        
        if (hasCameraPermission()) {
            startCamera()
        } else {
            requestCameraPermission()
        }
    }

    private fun initViews() {
        previewView = findViewById(R.id.previewView)
        statusText = findViewById(R.id.statusText)
        instructionText = findViewById(R.id.instructionText)
        closeButton = findViewById(R.id.closeButton)
        flashButton = findViewById(R.id.flashButton)
        progressBar = findViewById(R.id.progressBar)
        resultContainer = findViewById(R.id.resultContainer)
        resultTitle = findViewById(R.id.resultTitle)
        resultMessage = findViewById(R.id.resultMessage)
        confirmButton = findViewById(R.id.confirmButton)
        cancelButton = findViewById(R.id.cancelButton)
        scanLine = findViewById(R.id.scanLine)

        resultContainer.visibility = View.GONE
        progressBar.visibility = View.GONE
        
        // Start scan line animation after layout is ready
        scanLine.post {
            startScanLineAnimation()
        }
    }
    
    private fun startScanLineAnimation() {
        // Calculate animation distance (250dp frame height - line margins)
        val frameHeight = resources.getDimensionPixelSize(R.dimen.scan_frame_size) - 
                         resources.getDimensionPixelSize(R.dimen.scan_line_margin)
        
        scanLineAnimator = ObjectAnimator.ofFloat(scanLine, "translationY", 0f, frameHeight.toFloat()).apply {
            duration = 2000
            repeatCount = ValueAnimator.INFINITE
            repeatMode = ValueAnimator.REVERSE
            interpolator = LinearInterpolator()
            start()
        }
    }
    
    private fun stopScanLineAnimation() {
        scanLineAnimator?.cancel()
        scanLineAnimator = null
    }

    private fun setupListeners() {
        closeButton.setOnClickListener {
            finish()
        }

        flashButton.setOnClickListener {
            toggleFlash()
        }

        confirmButton.setOnClickListener {
            scannedQRData?.let { qrData ->
                confirmQRAuth(qrData)
            }
        }

        cancelButton.setOnClickListener {
            resetScanner()
        }
    }

    private fun hasCameraPermission(): Boolean {
        return ContextCompat.checkSelfPermission(
            this, Manifest.permission.CAMERA
        ) == PackageManager.PERMISSION_GRANTED
    }

    private fun requestCameraPermission() {
        ActivityCompat.requestPermissions(
            this,
            arrayOf(Manifest.permission.CAMERA),
            CAMERA_PERMISSION_CODE
        )
    }

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == CAMERA_PERMISSION_CODE) {
            if (grantResults.isNotEmpty() && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
                startCamera()
            } else {
                Toast.makeText(this, "Camera permission is required for QR scanning", Toast.LENGTH_LONG).show()
                finish()
            }
        }
    }

    private fun startCamera() {
        cameraExecutor = Executors.newSingleThreadExecutor()

        val cameraProviderFuture = ProcessCameraProvider.getInstance(this)
        cameraProviderFuture.addListener({
            try {
                val cameraProvider = cameraProviderFuture.get()
                bindCameraUseCases(cameraProvider)
            } catch (e: Exception) {
                Log.e(TAG, "Camera initialization failed", e)
                Toast.makeText(this, "Failed to start camera", Toast.LENGTH_SHORT).show()
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun bindCameraUseCases(cameraProvider: ProcessCameraProvider) {
        // Optimized preview for faster QR code scanning
        val preview = Preview.Builder()
            .setTargetResolution(Size(1280, 720))
            .build()
            .also {
                it.setSurfaceProvider(previewView.surfaceProvider)
            }

        // Optimized image analysis for fast QR detection
        val imageAnalyzer = ImageAnalysis.Builder()
            .setTargetResolution(Size(1280, 720))  // Higher resolution for better detection
            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
            .setOutputImageFormat(ImageAnalysis.OUTPUT_IMAGE_FORMAT_YUV_420_888)
            .build()
            .also {
                it.setAnalyzer(cameraExecutor!!, QRCodeAnalyzer { qrData ->
                    if (!isProcessing) {
                        runOnUiThread {
                            handleQRCodeScanned(qrData)
                        }
                    }
                })
            }

        val cameraSelector = CameraSelector.DEFAULT_BACK_CAMERA

        try {
            cameraProvider.unbindAll()
            camera = cameraProvider.bindToLifecycle(
                this, cameraSelector, preview, imageAnalyzer
            )
        } catch (e: Exception) {
            Log.e(TAG, "Use case binding failed", e)
        }
    }

    private fun toggleFlash() {
        camera?.let {
            if (it.cameraInfo.hasFlashUnit()) {
                isFlashOn = !isFlashOn
                it.cameraControl.enableTorch(isFlashOn)
                flashButton.setImageResource(
                    if (isFlashOn) R.drawable.ic_flash_on else R.drawable.ic_flash_off
                )
            }
        }
    }

    private fun handleQRCodeScanned(qrData: String) {
        if (isProcessing) return
        isProcessing = true
        
        Log.d(TAG, "QR Code scanned: $qrData")
        
        // Validate it's an NHP QR code
        if (!qrData.startsWith("nhp://scan?")) {
            statusText.text = "Invalid QR code. Please scan an NHP login QR."
            isProcessing = false
            return
        }

        scannedQRData = qrData
        
        // Parse QR data using nhpcore
        Thread {
            try {
                val parseResult = nhpcore.Nhpcore.parseQRCodeData(qrData)
                val result = org.json.JSONObject(parseResult)
                
                if (result.optBoolean("success", false)) {
                    // Notify server that QR code has been scanned
                    val serverUrl = result.optString("server", "")
                    val sessionId = result.optString("sessionId", "")
                    val aspId = result.optString("aspId", "example")
                    val resId = result.optString("resId", "demo")
                    
                    if (serverUrl.isNotEmpty() && sessionId.isNotEmpty()) {
                        Log.d(TAG, "Notifying server of QR scan: sessionId=$sessionId")
                        runOnUiThread {
                            statusText.text = "Notifying server..."
                        }
                        
                        // Call scan notification API
                        val scanResult = nhpcore.Nhpcore.notifyQRScan(serverUrl, sessionId, aspId, resId)
                        val scanResponse = org.json.JSONObject(scanResult)
                        
                        if (scanResponse.optBoolean("success", false)) {
                            Log.d(TAG, "Server notified successfully")
                        } else {
                            Log.w(TAG, "Scan notification failed: ${scanResponse.optString("errMsg")}")
                            // Continue anyway - this is just a status update
                        }
                    }
                    
                    runOnUiThread {
                        showConfirmDialog(result)
                    }
                } else {
                    runOnUiThread {
                        statusText.text = result.optString("errMsg", "Failed to parse QR code")
                        isProcessing = false
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "QR parse error", e)
                runOnUiThread {
                    statusText.text = "Error: ${e.message}"
                    isProcessing = false
                }
            }
        }.start()
    }

    private fun showConfirmDialog(parseResult: org.json.JSONObject) {
        previewView.alpha = 0.3f
        resultContainer.visibility = View.VISIBLE
        
        // Extract info for debug display - now using parsed server from QR data
        val token = parseResult.optString("token", "")
        val otpSecret = parseResult.optString("otpSecret", "")
        val serverUrl = parseResult.optString("server", "")
        val sessionId = parseResult.optString("sessionId", "")
        val aspId = parseResult.optString("aspId", "example")
        val resId = parseResult.optString("resId", "demo")
        
        resultTitle.text = "NHP Login"
        resultMessage.text = "Confirm to login?"
        
        statusText.text = "Scanned successfully"
        
        // Debug log (only in logcat)
        Log.d(TAG, "=== QR Code Parsed ===")
        Log.d(TAG, "Server URL (from QR): $serverUrl")
        Log.d(TAG, "Session ID: $sessionId")
        Log.d(TAG, "AspId: $aspId, ResId: $resId")
        Log.d(TAG, "======================")
    }

    private fun confirmQRAuth(qrData: String) {
        progressBar.visibility = View.VISIBLE
        confirmButton.isEnabled = false
        cancelButton.isEnabled = false
        statusText.text = "Authenticating..."

        Thread {
            try {
                // Parse QR data
                val parseResultStr = nhpcore.Nhpcore.parseQRCodeData(qrData)
                val parseResult = org.json.JSONObject(parseResultStr)
                
                if (!parseResult.optBoolean("success", false)) {
                    throw Exception(parseResult.optString("errMsg", "Failed to parse QR code"))
                }
                
                val encryptedData = parseResult.optString("token", "")
                val otpSecret = parseResult.optString("otpSecret", "")
                
                // Get server URL from parsed QR data (now embedded in QR code)
                var serverUrl = parseResult.optString("server", "")
                if (serverUrl.isEmpty()) {
                    // Fallback to extracting from raw QR data or use default
                    serverUrl = extractServerUrl(qrData)
                }
                
                // Get aspId and resId from parsed QR data
                val aspId = parseResult.optString("aspId", "example")
                val resId = parseResult.optString("resId", "demo")
                
                // Generate TOTP code
                val totpResultStr = nhpcore.Nhpcore.generateTOTP(otpSecret)
                val totpResult = org.json.JSONObject(totpResultStr)
                
                if (!totpResult.optBoolean("success", false)) {
                    throw Exception("Failed to generate OTP code")
                }
                
                val otpCode = totpResult.optString("code", "")
                
                // Get device info
                val deviceInfo = "${android.os.Build.MANUFACTURER} ${android.os.Build.MODEL}"
                
                // Build request URL using plugin system format
                val requestUrl = "$serverUrl/plugins/$aspId?resid=$resId&action=verify"
                
                // Debug log (only in logcat, not shown to user)
                Log.d(TAG, "=== QR Auth Debug Info ===")
                Log.d(TAG, "Server URL: $serverUrl")
                Log.d(TAG, "Request URL: $requestUrl")
                Log.d(TAG, "OTP Code: $otpCode")
                Log.d(TAG, "==========================")
                
                // Update UI with simple status
                runOnUiThread {
                    statusText.text = "Authenticating..."
                }
                
                // Verify with server using plugin system format
                val verifyResultStr = nhpcore.Nhpcore.verifyQRAuth(serverUrl, encryptedData, otpCode, deviceInfo, aspId, resId)
                val verifyResult = org.json.JSONObject(verifyResultStr)
                
                // Debug: Log response
                Log.d(TAG, "Verify Response: $verifyResultStr")
                
                runOnUiThread {
                    progressBar.visibility = View.GONE
                    
                    if (verifyResult.optBoolean("success", false)) {
                        resultTitle.text = "✓ Login Successful"
                        resultMessage.text = "You are now logged in."
                        statusText.text = "Success!"
                        
                        confirmButton.text = "Done"
                        confirmButton.isEnabled = true
                        confirmButton.setOnClickListener {
                            finish()
                        }
                        cancelButton.visibility = View.GONE
                    } else {
                        val errMsg = verifyResult.optString("errMsg", "Authentication failed")
                        resultTitle.text = "✗ Login Failed"
                        resultMessage.text = "Error: $errMsg"
                        statusText.text = "Failed"
                        
                        confirmButton.isEnabled = true
                        cancelButton.isEnabled = true
                        
                        Log.e(TAG, "Auth failed - URL: $requestUrl, Response: $verifyResultStr")
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "QR auth error", e)
                val errorDetails = """
                    |Error: ${e.message}
                    |
                    |QR Data: ${qrData.take(100)}...
                    |Stack: ${e.stackTraceToString().take(500)}
                """.trimMargin()
                
                runOnUiThread {
                    progressBar.visibility = View.GONE
                    confirmButton.isEnabled = true
                    cancelButton.isEnabled = true
                    
                    resultTitle.text = "✗ Error"
                    resultMessage.text = errorDetails
                    statusText.text = "Error: ${e.message?.take(50)}"
                }
            }
        }.start()
    }

    private fun extractServerUrl(qrData: String): String {
        // Parse the QR data to extract server URL
        // Format: nhp://scan?data=...&otp=...&server=...
        try {
            val queryStr = qrData.removePrefix("nhp://scan?")
            val params = queryStr.split("&").associate {
                val kv = it.split("=", limit = 2)
                if (kv.size == 2) kv[0] to java.net.URLDecoder.decode(kv[1], "UTF-8")
                else kv[0] to ""
            }
            return params["server"] ?: "https://nhp.opennhp.org/plugins/example?resid=demo&action=verify"
        } catch (e: Exception) {
            return "https://nhp.opennhp.org"
        }
    }

    private fun resetScanner() {
        isProcessing = false
        scannedQRData = null
        resultContainer.visibility = View.GONE
        previewView.alpha = 1.0f
        statusText.text = "Point camera at QR code"
        confirmButton.text = "Confirm"
        confirmButton.isEnabled = true
        cancelButton.visibility = View.VISIBLE
        cancelButton.isEnabled = true
    }

    override fun onDestroy() {
        super.onDestroy()
        stopScanLineAnimation()
        cameraExecutor?.shutdown()
    }

    /**
     * QR Code Analyzer using ML Kit - Optimized for fast scanning
     */
    private inner class QRCodeAnalyzer(
        private val onQRCodeDetected: (String) -> Unit
    ) : ImageAnalysis.Analyzer {

        // Optimized scanner options for faster QR detection
        private val options = BarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .enableAllPotentialBarcodes()  // Detect barcodes even if partially visible
            .build()
        
        private val scanner: BarcodeScanner = BarcodeScanning.getClient(options)
        
        @Volatile
        private var isScanning = false

        @androidx.camera.core.ExperimentalGetImage
        override fun analyze(imageProxy: ImageProxy) {
            // Skip if already scanning to prevent queue buildup
            if (isScanning) {
                imageProxy.close()
                return
            }
            
            val mediaImage = imageProxy.image
            if (mediaImage != null) {
                isScanning = true
                val image = InputImage.fromMediaImage(
                    mediaImage, imageProxy.imageInfo.rotationDegrees
                )
                
                scanner.process(image)
                    .addOnSuccessListener { barcodes ->
                        for (barcode in barcodes) {
                            barcode.rawValue?.let { value ->
                                onQRCodeDetected(value)
                                return@addOnSuccessListener
                            }
                        }
                    }
                    .addOnFailureListener { e ->
                        Log.e(TAG, "Barcode scanning failed", e)
                    }
                    .addOnCompleteListener {
                        isScanning = false
                        imageProxy.close()
                    }
            } else {
                imageProxy.close()
            }
        }
    }
}

