package org.opennhp.stealthdns.browser

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.net.Uri
import android.os.Bundle
import android.util.Log
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.webkit.*
import android.widget.*
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.swiperefreshlayout.widget.SwipeRefreshLayout

/**
 * Main browser activity with NHP protocol support
 */
class MainActivity : AppCompatActivity() {

    companion object {
        private const val TAG = "StealthDNS"
    }

    private var webView: WebView? = null
    private var urlEditText: EditText? = null
    private var progressBar: ProgressBar? = null
    private var swipeRefresh: SwipeRefreshLayout? = null
    private var nhpIndicator: LinearLayout? = null
    private var nhpStatusText: TextView? = null
    private var btnBack: ImageButton? = null
    private var btnForward: ImageButton? = null
    private var btnRefresh: ImageButton? = null
    private var btnQrScan: ImageButton? = null
    private var btnHome: ImageButton? = null
    private var btnTabs: ImageButton? = null
    private var btnMenu: ImageButton? = null
    private var securityIcon: ImageView? = null

    // Track if current page was loaded via NHP knock
    private var currentPageIsNhp = false
    private var nhpInitialized = false
    
    // Map: real URL (with resolved host) -> original NHP URL (with .nhp domain)
    // Used to restore NHP display URL when navigating back/forward
    private val nhpUrlMapping = mutableMapOf<String, String>()
    
    // Current original NHP URL being displayed (to show in address bar instead of real URL)
    private var currentNhpDisplayUrl: String? = null
    
    // Tab management
    data class BrowserTab(
        val id: Int,
        var title: String = "New Tab",
        var url: String = "https://demo.nhp",
        var isNhp: Boolean = false
    )
    
    private val tabs = mutableListOf<BrowserTab>()
    private var currentTabIndex = 0
    private var tabIdCounter = 0

    // Pending URL to load after NHP initialization
    private var pendingDefaultUrl: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Log.d(TAG, "onCreate started")
        
        try {
            setContentView(R.layout.activity_main)
            initViews()
            setupWebView()
            setupListeners()
            
            // Create first tab
            createNewTab()
            
            // Handle intent or load default page
            val intentUrl = intent?.data?.toString()
            val urlToLoad = if (!intentUrl.isNullOrEmpty()) intentUrl else "https://demo.nhp"
            
            // Initialize NHP in background and load URL after initialization
            initializeNHPCoreAsync(urlToLoad)
            
        } catch (e: Exception) {
            Log.e(TAG, "Error in onCreate", e)
            Toast.makeText(this, "Startup error: ${e.message}", Toast.LENGTH_LONG).show()
        }
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent)
        intent?.data?.let { uri ->
            loadUrl(uri.toString())
        }
    }

    private fun initializeNHPCoreAsync(urlToLoadAfterInit: String) {
        // Show initialization progress
        runOnUiThread {
            nhpIndicator?.visibility = View.VISIBLE
            nhpStatusText?.text = "Initializing NHP..."
            progressBar?.visibility = View.VISIBLE
        }
        
        Thread {
            try {
                val workDir = filesDir.absolutePath
                
                // Create necessary directories
                java.io.File(workDir, "etc").mkdirs()
                java.io.File(workDir, "logs").mkdirs()
                
                // Copy config files from assets to workDir
                copyConfigFiles(workDir)
                
                // Load config JSON
                val configJson = assets.open("resources.json").bufferedReader().use { it.readText() }
                Log.d(TAG, "Config JSON loaded")
                
                // Initialize with config (only method available now)
                nhpcore.Nhpcore.initializeWithConfig(workDir, 4L, configJson)
                nhpInitialized = true
                Log.d(TAG, "NHP initialized successfully")
                
                // Now load the default URL on main thread
                runOnUiThread {
                    loadUrl(urlToLoadAfterInit)
                }
                
            } catch (e: Exception) {
                Log.e(TAG, "NHP initialization failed", e)
                nhpInitialized = false
                
                // Still try to load URL (will show error for NHP domains)
                runOnUiThread {
                    nhpStatusText?.text = "NHP initialization failed"
                    loadUrl(urlToLoadAfterInit)
                }
            }
        }.start()
    }
    
    private fun copyConfigFiles(workDir: String) {
        try {
            val etcDir = java.io.File(workDir, "etc")
            val configJson = assets.open("resources.json").bufferedReader().use { it.readText() }
            val config = org.json.JSONObject(configJson)
            
            // Create config.toml
            val agent = config.optJSONObject("agent")
            if (agent != null) {
                val configToml = StringBuilder()
                configToml.append("PrivateKeyBase64 = \"${agent.optString("privateKeyBase64", "")}\"\n")
                configToml.append("DefaultCipherScheme = ${agent.optInt("cipherScheme", 1)}\n")
                configToml.append("UserId = \"${agent.optString("userId", "mobile-user")}\"\n")
                configToml.append("OrganizationId = \"${agent.optString("organizationId", "")}\"\n")
                configToml.append("LogLevel = 4\n")
                java.io.File(etcDir, "config.toml").writeText(configToml.toString())
            }
            
            // Create server.toml
            val servers = config.optJSONArray("servers")
            if (servers != null && servers.length() > 0) {
                val serverToml = StringBuilder()
                for (i in 0 until servers.length()) {
                    val srv = servers.getJSONObject(i)
                    serverToml.append("[[Servers]]\n")
                    serverToml.append("Hostname = \"${srv.optString("hostname", "")}\"\n")
                    serverToml.append("Ip = \"${srv.optString("ip", "")}\"\n")
                    serverToml.append("Port = ${srv.optInt("port", 62206)}\n")
                    serverToml.append("PubKeyBase64 = \"${srv.optString("pubKeyBase64", "")}\"\n")
                    serverToml.append("ExpireTime = ${srv.optLong("expireTime", 0)}\n\n")
                }
                java.io.File(etcDir, "server.toml").writeText(serverToml.toString())
            }
            
            // Create resource.toml
            val resources = config.optJSONArray("resources")
            if (resources != null && resources.length() > 0) {
                val resourceToml = StringBuilder()
                for (i in 0 until resources.length()) {
                    val res = resources.getJSONObject(i)
                    resourceToml.append("[[Resources]]\n")
                    resourceToml.append("AuthServiceId = \"${res.optString("authServiceId", "")}\"\n")
                    resourceToml.append("ResourceId = \"${res.optString("resourceId", "")}\"\n")
                    resourceToml.append("ServerHostname = \"${res.optString("serverHostname", "")}\"\n")
                    resourceToml.append("ServerIp = \"${res.optString("serverIp", "")}\"\n")
                    resourceToml.append("ServerPort = ${res.optInt("serverPort", 62206)}\n\n")
                }
                java.io.File(etcDir, "resource.toml").writeText(resourceToml.toString())
            }
            
            Log.d(TAG, "Config files created")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to copy config files", e)
        }
    }

    private fun initViews() {
        webView = findViewById(R.id.webView)
        urlEditText = findViewById(R.id.urlEditText)
        progressBar = findViewById(R.id.progressBar)
        swipeRefresh = findViewById(R.id.swipeRefresh)
        nhpIndicator = findViewById(R.id.nhpIndicator)
        nhpStatusText = findViewById(R.id.nhpStatusText)
        btnBack = findViewById(R.id.btnBack)
        btnForward = findViewById(R.id.btnForward)
        btnRefresh = findViewById(R.id.btnRefresh)
        btnQrScan = findViewById(R.id.btnQrScan)
        btnHome = findViewById(R.id.btnHome)
        btnTabs = findViewById(R.id.btnTabs)
        btnMenu = findViewById(R.id.btnMenu)
        securityIcon = findViewById(R.id.securityIcon)
        
        // Initially hide NHP indicator
        nhpIndicator?.visibility = View.GONE
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun setupWebView() {
        webView?.settings?.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            setSupportZoom(true)
            builtInZoomControls = true
            displayZoomControls = false
            loadWithOverviewMode = true
            useWideViewPort = true
            allowFileAccess = true
            allowContentAccess = true
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            cacheMode = WebSettings.LOAD_DEFAULT
            userAgentString = userAgentString + " StealthDNS/1.0"
        }

        webView?.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
                val url = request?.url?.toString() ?: return false
                val scheme = request.url.scheme?.lowercase() ?: ""
                val host = request.url.host ?: ""
                
                Log.d(TAG, "shouldOverrideUrlLoading: url=$url, scheme=$scheme, host=$host")
                
                // Allow all standard web protocols to be handled by WebView
                if (scheme == "http" || scheme == "https") {
                    // Check if it's an NHP domain
                    if (host.isNotEmpty() && nhpInitialized && isNHPDomain(host)) {
                        Log.d(TAG, "Detected NHP domain: $host")
                        processNHPUrl(url, host)
                        return true
                    }
                    
                    // For normal HTTP/HTTPS URLs, let WebView handle them
                    // Only clear NHP status if user is navigating away (not internal redirect)
                    // Check if this is a user-initiated navigation vs server redirect
                    if (!currentPageIsNhp) {
                        // Not in NHP mode, just let WebView handle
                        return false
                    }
                    
                    // If current page is NHP protected, this might be a server redirect
                    // Keep NHP status and let WebView handle the redirect
                    Log.d(TAG, "NHP page redirect/navigation: $url, keeping NHP status")
                    return false
                }
                
                // Ignore special schemes (javascript, about, data, blob)
                if (scheme == "javascript" || scheme == "about" || scheme == "data" || scheme == "blob") {
                    Log.d(TAG, "Ignoring special scheme: $scheme")
                    return false
                }
                
                // Handle intent:// URLs (open external apps)
                if (scheme == "intent") {
                    try {
                        val intent = Intent.parseUri(url, Intent.URI_INTENT_SCHEME)
                        if (intent.resolveActivity(packageManager) != null) {
                            startActivity(intent)
                        } else {
                            Log.d(TAG, "No app to handle intent: $url")
                        }
                    } catch (e: Exception) {
                        Log.e(TAG, "Failed to handle intent URL: $url", e)
                    }
                    return true
                }
                
                // Handle common external app schemes (tel, mailto, sms, etc.)
                if (scheme == "tel" || scheme == "mailto" || scheme == "sms" || scheme == "market" || scheme == "geo") {
                    try {
                        val intent = Intent(Intent.ACTION_VIEW, request.url)
                        if (intent.resolveActivity(packageManager) != null) {
                            startActivity(intent)
                        }
                    } catch (e: Exception) {
                        Log.e(TAG, "Failed to handle external URL: $url", e)
                    }
                    return true
                }
                
                // Handle custom app schemes (baiduboxapp, weixin, alipay, etc.)
                // Try to open the app, if not installed, silently ignore
                if (!scheme.startsWith("http")) {
                    Log.d(TAG, "Custom scheme detected: $scheme, attempting to open app")
                    try {
                        val intent = Intent(Intent.ACTION_VIEW, request.url)
                        if (intent.resolveActivity(packageManager) != null) {
                            startActivity(intent)
                        } else {
                            // App not installed, silently ignore
                            Log.d(TAG, "No app installed for scheme: $scheme")
                        }
                    } catch (e: Exception) {
                        Log.d(TAG, "Cannot handle custom scheme: $scheme")
                    }
                    // Always return true to prevent WebView from showing error
                    return true
                }
                
                // For any other cases, let WebView handle
                return false
            }

            override fun onPageStarted(view: WebView?, url: String?, favicon: Bitmap?) {
                super.onPageStarted(view, url, favicon)
                progressBar?.visibility = View.VISIBLE
                swipeRefresh?.isRefreshing = true
                url?.let {
                    // Check if this URL was loaded via NHP knock (for back/forward navigation)
                    val nhpUrl = nhpUrlMapping[it]
                    if (nhpUrl != null) {
                        // Restore NHP state when navigating back to NHP page
                        currentPageIsNhp = true
                        currentNhpDisplayUrl = nhpUrl
                        urlEditText?.setText(nhpUrl)
                    } else if (currentPageIsNhp && currentNhpDisplayUrl != null) {
                        // Still in NHP mode (e.g., redirect within NHP page)
                        urlEditText?.setText(currentNhpDisplayUrl)
                    } else {
                        urlEditText?.setText(it)
                    }
                    
                    updateSecurityIndicator(it)
                }
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                progressBar?.visibility = View.GONE
                swipeRefresh?.isRefreshing = false
                updateNavigationButtons()
                
                url?.let {
                    // Check if this URL was loaded via NHP knock (for back/forward navigation)
                    val nhpUrl = nhpUrlMapping[it]
                    val displayUrl: String
                    
                    if (nhpUrl != null) {
                        // Restore NHP state when navigating back to NHP page
                        currentPageIsNhp = true
                        currentNhpDisplayUrl = nhpUrl
                        displayUrl = nhpUrl
                    } else if (currentPageIsNhp && currentNhpDisplayUrl != null) {
                        // Still in NHP mode (e.g., redirect within NHP page)
                        // Also save this redirected URL to mapping for back/forward navigation
                        nhpUrlMapping[it] = currentNhpDisplayUrl!!
                        Log.d(TAG, "Added redirect URL to mapping: $it -> ${currentNhpDisplayUrl}")
                        displayUrl = currentNhpDisplayUrl!!
                    } else {
                        // Normal non-NHP page
                        currentPageIsNhp = false
                        currentNhpDisplayUrl = null
                        displayUrl = it
                    }
                    
                    urlEditText?.setText(displayUrl)
                    
                    // Update current tab info
                    if (tabs.isNotEmpty() && currentTabIndex < tabs.size) {
                        tabs[currentTabIndex].title = view?.title ?: "New Tab"
                        tabs[currentTabIndex].url = displayUrl  // Store the display URL (NHP URL)
                        tabs[currentTabIndex].isNhp = currentPageIsNhp
                    }
                }
            }

            override fun onReceivedError(view: WebView?, request: WebResourceRequest?, error: WebResourceError?) {
                super.onReceivedError(view, request, error)
                val errorCode = error?.errorCode ?: -1
                val errorDesc = error?.description?.toString() ?: "Unknown error"
                val failedUrl = request?.url?.toString() ?: ""
                
                Log.e(TAG, "WebView error: code=$errorCode, desc=$errorDesc, url=$failedUrl, isMainFrame=${request?.isForMainFrame}")
                
                // Only handle main frame errors (not resource loading errors)
                if (request?.isForMainFrame != true) {
                    return
                }
                
                // Ignore certain error codes that are not real errors
                // ERROR_UNKNOWN (-1), ERROR_HOST_LOOKUP (-2), ERROR_CONNECT (-6), ERROR_TIMEOUT (-8)
                // ERROR_BAD_URL (-12), ERROR_UNSUPPORTED_SCHEME (-10)
                when (errorCode) {
                    WebViewClient.ERROR_UNSUPPORTED_SCHEME -> {
                        // Unsupported scheme, might be intent:// or other app links
                        Log.d(TAG, "Unsupported scheme, ignoring error page")
                        return
                    }
                    WebViewClient.ERROR_UNKNOWN -> {
                        // Unknown error, might be cancelled navigation
                        Log.d(TAG, "Unknown error, ignoring error page")
                        return
                    }
                }
                
                progressBar?.visibility = View.GONE
                swipeRefresh?.isRefreshing = false
                
                // On error, hide NHP indicator
                if (currentPageIsNhp) {
                    currentPageIsNhp = false
                    nhpIndicator?.visibility = View.GONE
                }
                
                // Show custom error page for main frame errors
                val errorHtml = """
                    <html>
                    <head>
                        <meta name="viewport" content="width=device-width, initial-scale=1.0">
                        <style>
                            body { 
                                font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
                                background: #0b1020; 
                                color: #fff; 
                                display: flex; 
                                flex-direction: column;
                                align-items: center; 
                                justify-content: center; 
                                height: 100vh; 
                                margin: 0;
                                padding: 20px;
                                box-sizing: border-box;
                                text-align: center;
                            }
                            .icon { font-size: 64px; margin-bottom: 20px; }
                            h1 { font-size: 20px; margin: 0 0 10px 0; color: #ff9800; }
                            p { font-size: 14px; color: #9e9e9e; margin: 5px 0; word-break: break-all; }
                            .url { font-size: 12px; color: #666; margin-top: 15px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
                            button { 
                                margin-top: 20px; 
                                padding: 12px 24px; 
                                background: #29b6f6; 
                                border: none; 
                                border-radius: 8px; 
                                color: white; 
                                font-size: 16px;
                                cursor: pointer;
                            }
                        </style>
                    </head>
                    <body>
                        <div class="icon">🌐</div>
                        <h1>Page Load Failed</h1>
                        <p>Error: $errorDesc</p>
                        <p class="url">$failedUrl</p>
                        <button onclick="window.location.href='https://demo.nhp'">Go Back</button>
                    </body>
                    </html>
                """.trimIndent()
                view?.loadDataWithBaseURL(null, errorHtml, "text/html", "UTF-8", null)
            }
            
            override fun onReceivedSslError(view: WebView?, handler: SslErrorHandler?, error: android.net.http.SslError?) {
                Log.e(TAG, "SSL Error: $error")
                handler?.proceed()
            }
        }

        webView?.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                super.onProgressChanged(view, newProgress)
                progressBar?.progress = newProgress
                if (newProgress == 100) {
                    progressBar?.visibility = View.GONE
                }
            }
        }

        if (BuildConfig.DEBUG) {
            WebView.setWebContentsDebuggingEnabled(true)
        }
    }

    private fun setupListeners() {
        urlEditText?.setOnEditorActionListener { textView, actionId, event ->
            when {
                actionId == EditorInfo.IME_ACTION_GO ||
                actionId == EditorInfo.IME_ACTION_DONE ||
                actionId == EditorInfo.IME_ACTION_SEARCH ||
                (event != null && event.keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_DOWN) -> {
                    hideKeyboard()
                    loadUrl(textView.text?.toString() ?: "")
                    true
                }
                else -> false
            }
        }
        
        urlEditText?.setOnKeyListener { _, keyCode, event ->
            if (keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_UP) {
                hideKeyboard()
                loadUrl(urlEditText?.text?.toString() ?: "")
                true
            } else {
                false
            }
        }

        btnBack?.setOnClickListener {
            if (webView?.canGoBack() == true) webView?.goBack()
        }

        btnForward?.setOnClickListener {
            if (webView?.canGoForward() == true) webView?.goForward()
        }

        btnRefresh?.setOnClickListener {
            webView?.reload()
        }

        btnQrScan?.setOnClickListener {
            openQRScanner()
        }

        btnHome?.setOnClickListener {
            currentPageIsNhp = false
            nhpIndicator?.visibility = View.GONE
            loadUrl("https://demo.nhp")
        }

        btnTabs?.setOnClickListener {
            showTabsInfo()
        }

        btnMenu?.setOnClickListener {
            showMenu()
        }

        swipeRefresh?.setOnRefreshListener {
            webView?.reload()
        }

        swipeRefresh?.setColorSchemeColors(
            ContextCompat.getColor(this, R.color.nhp_blue)
        )
    }
    
    private fun hideKeyboard() {
        try {
            val imm = getSystemService(Context.INPUT_METHOD_SERVICE) as? InputMethodManager
            imm?.hideSoftInputFromWindow(urlEditText?.windowToken, 0)
            urlEditText?.clearFocus()
        } catch (e: Exception) {
            Log.e(TAG, "Error hiding keyboard", e)
        }
    }

    private fun showTabsInfo() {
        // Save current tab state first
        saveCurrentTabState()
        
        // Build tab list for display
        val tabItems = tabs.mapIndexed { index, tab ->
            val prefix = if (index == currentTabIndex) "▶ " else "   "
            val nhpBadge = if (tab.isNhp) " 🛡️" else ""
            val title = tab.title.take(25) + if (tab.title.length > 25) "..." else ""
            "$prefix$title$nhpBadge"
        }.toMutableList()
        
        // Add action items
        tabItems.add("──────────────")
        tabItems.add("➕ New Tab")
        tabItems.add("🗑️ Close Current Tab")
        tabItems.add("🧹 Clear Browsing Data")
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Tabs (${tabs.size})")
        builder.setItems(tabItems.toTypedArray()) { _, which ->
            when {
                which < tabs.size -> {
                    // Switch to selected tab
                    switchToTab(which)
                }
                which == tabs.size + 1 -> {
                    // New tab in background (stay on current page)
                    createNewTabInBackground()
                    Toast.makeText(this, "New tab created (${tabs.size})", Toast.LENGTH_SHORT).show()
                }
                which == tabs.size + 2 -> {
                    // Close current tab
                    closeCurrentTab()
                }
                which == tabs.size + 3 -> {
                    // Clear browsing data
                    clearBrowsingData()
                }
            }
        }
        builder.show()
    }
    
    // Create new tab in background without switching
    private fun createNewTabInBackground() {
        // Save current tab state first
        saveCurrentTabState()
        
        val newTab = BrowserTab(
            id = tabIdCounter++,
            title = "New Tab",
            url = "https://demo.nhp"
        )
        tabs.add(newTab)
        // Don't change currentTabIndex - stay on current tab
        
        updateTabIndicator()
    }
    
    // Create new tab and switch to it
    private fun createNewTabAndSwitch() {
        // Save current tab state first
        saveCurrentTabState()
        
        val newTab = BrowserTab(
            id = tabIdCounter++,
            title = "New Tab",
            url = "https://demo.nhp"
        )
        tabs.add(newTab)
        currentTabIndex = tabs.size - 1
        
        // Load home page for new tab
        currentPageIsNhp = false
        nhpIndicator?.visibility = View.GONE
        urlEditText?.setText("")
        
        // Load the new tab's URL
        webView?.stopLoading()
        webView?.loadUrl("https://demo.nhp")
        
        updateTabIndicator()
    }
    
    // Used only for initial tab creation
    private fun createNewTab(): BrowserTab {
        val newTab = BrowserTab(
            id = tabIdCounter++,
            title = "New Tab",
            url = "https://demo.nhp"
        )
        tabs.add(newTab)
        currentTabIndex = tabs.size - 1
        updateTabIndicator()
        return newTab
    }
    
    private fun saveCurrentTabState() {
        if (tabs.isEmpty() || currentTabIndex >= tabs.size) return
        
        val currentTab = tabs[currentTabIndex]
        currentTab.title = webView?.title ?: "New Tab"
        currentTab.url = webView?.url ?: "https://demo.nhp"
        currentTab.isNhp = currentPageIsNhp
        
        // Note: We don't use saveState/restoreState as it's unreliable
        // Instead we reload the URL when switching tabs
    }
    
    private fun switchToTab(index: Int) {
        if (index == currentTabIndex || index >= tabs.size) return
        
        // Save current tab state first
        saveCurrentTabState()
        
        // Switch to new tab
        currentTabIndex = index
        val tab = tabs[index]
        
        // Stop current loading and load the tab's URL
        webView?.stopLoading()
        currentPageIsNhp = tab.isNhp
        urlEditText?.setText(tab.url)
        
        // Check if URL was loaded via NHP (tab.url is display URL, might be NHP URL)
        if (tab.isNhp || nhpUrlMapping.containsValue(tab.url)) {
            currentPageIsNhp = true
            currentNhpDisplayUrl = tab.url
        }
        
        // Load the URL - if it's an NHP URL, we need to reload via knock
        val url = tab.url
        if (url.isNotEmpty() && url != "about:blank") {
            loadUrl(url)  // Use loadUrl to handle NHP domains properly
        } else {
            loadUrl("https://demo.nhp")
        }
        
        updateSecurityIndicator(tab.url)
        updateTabIndicator()
        Toast.makeText(this, "Switched to: ${tab.title}", Toast.LENGTH_SHORT).show()
    }
    
    private fun closeCurrentTab() {
        if (tabs.size <= 1) {
            Toast.makeText(this, "At least one tab required", Toast.LENGTH_SHORT).show()
            return
        }
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Close Tab")
        builder.setMessage("Close this tab?\n\n${tabs[currentTabIndex].title}")
        builder.setPositiveButton("Close") { _, _ ->
            tabs.removeAt(currentTabIndex)
            
            // Adjust current index
            if (currentTabIndex >= tabs.size) {
                currentTabIndex = tabs.size - 1
            }
            
            // Load the new current tab
            val tab = tabs[currentTabIndex]
            webView?.stopLoading()
            currentPageIsNhp = tab.isNhp || nhpUrlMapping.containsValue(tab.url)
            if (currentPageIsNhp) {
                currentNhpDisplayUrl = tab.url
            }
            urlEditText?.setText(tab.url)
            
            val url = tab.url
            if (url.isNotEmpty() && url != "about:blank") {
                webView?.loadUrl(url)
            } else {
                webView?.loadUrl("https://demo.nhp")
            }
            
            updateSecurityIndicator(tab.url)
            updateTabIndicator()
            Toast.makeText(this, "Tab closed, ${tabs.size} remaining", Toast.LENGTH_SHORT).show()
        }
        builder.setNegativeButton("Cancel", null)
        builder.show()
    }
    
    private fun updateTabIndicator() {
        // Update the tabs button to show tab count (optional visual feedback)
        // For now, just log
        Log.d(TAG, "Current tabs: ${tabs.size}, active: ${currentTabIndex + 1}")
    }
    
    private fun clearBrowsingData() {
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Clear Browsing Data")
        builder.setMessage("Clear all browsing data?\n\nThis will clear:\n• Browsing history\n• Cache\n• Cookies\n• All tabs")
        builder.setPositiveButton("Clear") { _, _ ->
            webView?.clearHistory()
            webView?.clearCache(true)
            android.webkit.CookieManager.getInstance().removeAllCookies(null)
            nhpUrlMapping.clear()
            currentPageIsNhp = false
            nhpIndicator?.visibility = View.GONE
            
            // Reset tabs
            tabs.clear()
            tabIdCounter = 0
            createNewTab()
            loadUrl("https://demo.nhp")
            
            Toast.makeText(this, "Browsing data cleared", Toast.LENGTH_SHORT).show()
        }
        builder.setNegativeButton("Cancel", null)
        builder.show()
    }

    private fun showMenu() {
        val menuItems = arrayOf(
            "📷 Scan QR Code",
            "🔗 Share Page",
            "⭐ Add Bookmark",
            "📚 View Bookmarks",
            "📋 Copy Link",
            "🔍 Find in Page",
            "🖥️ Desktop Site",
            "ℹ️ About StealthDNS"
        )
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Menu")
        builder.setItems(menuItems) { _, which ->
            when (which) {
                0 -> openQRScanner()
                1 -> shareCurrentPage()
                2 -> addBookmark()
                3 -> showBookmarks()
                4 -> copyCurrentUrl()
                5 -> showFindInPage()
                6 -> toggleDesktopMode()
                7 -> showAboutDialog()
            }
        }
        builder.show()
    }

    private fun openQRScanner() {
        val intent = Intent(this, QRScanActivity::class.java)
        startActivity(intent)
    }

    private fun shareCurrentPage() {
        val url = webView?.url ?: return
        val title = webView?.title ?: "StealthDNS"
        
        val shareIntent = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_SUBJECT, title)
            putExtra(Intent.EXTRA_TEXT, "$title\n$url")
        }
        startActivity(Intent.createChooser(shareIntent, "Share via"))
    }

    private fun addBookmark() {
        val url = webView?.url ?: return
        val title = webView?.title ?: "Untitled"
        
        // Save bookmark to SharedPreferences
        val prefs = getSharedPreferences("bookmarks", Context.MODE_PRIVATE)
        val bookmarks = prefs.getStringSet("urls", mutableSetOf())?.toMutableSet() ?: mutableSetOf()
        
        // Check if already bookmarked
        val existing = bookmarks.find { it.contains("|$url") }
        if (existing != null) {
            Toast.makeText(this, "Page already bookmarked", Toast.LENGTH_SHORT).show()
            return
        }
        
        bookmarks.add("$title|$url")
        prefs.edit().putStringSet("urls", bookmarks).apply()
        
        Toast.makeText(this, "Bookmark added: $title", Toast.LENGTH_SHORT).show()
    }
    
    private fun showBookmarks() {
        val prefs = getSharedPreferences("bookmarks", Context.MODE_PRIVATE)
        val bookmarks = prefs.getStringSet("urls", mutableSetOf())?.toList() ?: emptyList()
        
        if (bookmarks.isEmpty()) {
            Toast.makeText(this, "No bookmarks", Toast.LENGTH_SHORT).show()
            return
        }
        
        // Parse bookmarks into display titles
        val titles = bookmarks.map { bookmark ->
            val parts = bookmark.split("|", limit = 2)
            val title = parts.getOrNull(0) ?: "Untitled"
            title.take(40) + if (title.length > 40) "..." else ""
        }.toTypedArray()
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Bookmarks (${bookmarks.size})")
        builder.setItems(titles) { _, which ->
            val bookmark = bookmarks[which]
            val parts = bookmark.split("|", limit = 2)
            val url = parts.getOrNull(1) ?: return@setItems
            loadUrl(url)
        }
        builder.setNeutralButton("Manage") { _, _ ->
            showBookmarkManager(bookmarks)
        }
        builder.setNegativeButton("Cancel", null)
        builder.show()
    }
    
    private fun showBookmarkManager(bookmarks: List<String>) {
        if (bookmarks.isEmpty()) {
            Toast.makeText(this, "No bookmarks", Toast.LENGTH_SHORT).show()
            return
        }
        
        val titles = bookmarks.map { bookmark ->
            val parts = bookmark.split("|", limit = 2)
            val title = parts.getOrNull(0) ?: "Untitled"
            "🗑️ " + title.take(35) + if (title.length > 35) "..." else ""
        }.toTypedArray()
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Delete Bookmarks")
        builder.setItems(titles) { _, which ->
            val bookmarkToRemove = bookmarks[which]
            val prefs = getSharedPreferences("bookmarks", Context.MODE_PRIVATE)
            val currentBookmarks = prefs.getStringSet("urls", mutableSetOf())?.toMutableSet() ?: mutableSetOf()
            currentBookmarks.remove(bookmarkToRemove)
            prefs.edit().putStringSet("urls", currentBookmarks).apply()
            Toast.makeText(this, "Bookmark deleted", Toast.LENGTH_SHORT).show()
        }
        builder.setPositiveButton("Clear All") { _, _ ->
            val prefs = getSharedPreferences("bookmarks", Context.MODE_PRIVATE)
            prefs.edit().remove("urls").apply()
            Toast.makeText(this, "All bookmarks cleared", Toast.LENGTH_SHORT).show()
        }
        builder.setNegativeButton("Cancel", null)
        builder.show()
    }

    private fun copyCurrentUrl() {
        val url = webView?.url ?: return
        
        val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
        val clip = android.content.ClipData.newPlainText("URL", url)
        clipboard.setPrimaryClip(clip)
        
        Toast.makeText(this, "Link copied", Toast.LENGTH_SHORT).show()
    }

    private fun showFindInPage() {
        // Simple find in page implementation using EditText dialog
        val editText = EditText(this).apply {
            hint = "Enter search text"
            setPadding(48, 32, 48, 32)
        }
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("Find in Page")
        builder.setView(editText)
        builder.setPositiveButton("Find") { _, _ ->
            val query = editText.text.toString()
            if (query.isNotEmpty()) {
                webView?.findAllAsync(query)
            }
        }
        builder.setNegativeButton("Cancel", null)
        builder.setNeutralButton("Clear") { _, _ ->
            webView?.clearMatches()
        }
        builder.show()
    }

    private var desktopMode = false
    
    private fun toggleDesktopMode() {
        desktopMode = !desktopMode
        
        webView?.settings?.apply {
            if (desktopMode) {
                userAgentString = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
                useWideViewPort = true
                loadWithOverviewMode = true
            } else {
                userAgentString = WebSettings.getDefaultUserAgent(this@MainActivity) + " StealthDNS/1.0"
                useWideViewPort = true
                loadWithOverviewMode = true
            }
        }
        
        webView?.reload()
        Toast.makeText(this, if (desktopMode) "Desktop mode enabled" else "Mobile mode enabled", Toast.LENGTH_SHORT).show()
    }

    private fun showAboutDialog() {
        val nhpStatus = if (nhpInitialized) "Initialized ✓" else "Not initialized"
        val message = """
            |StealthDNS Browser
            |Version: 1.0.0
            |
            |A secure browser based on NHP (Network Hiding Protocol)
            |
            |NHP Status: $nhpStatus
            |
            |© 2024 OpenNHP
        """.trimMargin()
        
        val builder = android.app.AlertDialog.Builder(this, R.style.AlertDialogTheme)
        builder.setTitle("About")
        builder.setMessage(message)
        builder.setPositiveButton("OK", null)
        builder.show()
    }

    private fun loadUrl(input: String) {
        var url = input.trim()
        if (url.isEmpty()) return

        // Reset NHP status when loading new URL
        currentPageIsNhp = false
        currentNhpDisplayUrl = null
        nhpIndicator?.visibility = View.GONE

        // Determine if it's a search query or URL
        if (!url.contains(".") || url.contains(" ")) {
            url = "https://demo.nhp/s?wd=" + Uri.encode(url)
        } else if (!url.startsWith("http://") && !url.startsWith("https://")) {
            url = "https://$url"
        }

        val uri = Uri.parse(url)
        val host = uri.host ?: ""

        // Check if it's an NHP domain
        if (nhpInitialized && isNHPDomain(host)) {
            processNHPUrl(url, host)
        } else {
            webView?.loadUrl(url)
        }
    }

    private fun isNHPDomain(host: String): Boolean {
        return try {
            nhpcore.Nhpcore.isNHPDomain(host)
        } catch (e: Exception) {
            host.lowercase().endsWith(".nhp")
        }
    }

    private fun processNHPUrl(originalUrl: String, host: String) {
        Log.d(TAG, "processNHPUrl: $originalUrl, host: $host")
        
        runOnUiThread {
            progressBar?.visibility = View.VISIBLE
            nhpIndicator?.visibility = View.VISIBLE
            nhpStatusText?.text = "Performing NHP knock..."
        }

        Thread {
            try {
                val resourceId = nhpcore.Nhpcore.extractResourceID(host)
                Log.d(TAG, "Extracted resourceId: $resourceId")
                
                val resultJson = nhpcore.Nhpcore.getKnockResultJSON(resourceId)
                Log.d(TAG, "Knock result: $resultJson")
                
                // Parse ServerKnockAckMsg format
                // Fields: errCode, errMsg, resHost (map), opnTime, agentAddr, etc.
                val result = org.json.JSONObject(resultJson)
                val errCode = result.optString("errCode", "")
                val errMsg = result.optString("errMsg", "")
                val resHost = result.optJSONObject("resHost")
                val openTime = result.optInt("opnTime", 0)
                
                // Success if errCode is "0" or empty
                val success = (errCode == "0" || errCode.isEmpty()) && resHost != null && resHost.length() > 0
                
                // Get first resource host from map
                var resolvedHost = ""
                if (resHost != null) {
                    val keys = resHost.keys()
                    if (keys.hasNext()) {
                        resolvedHost = resHost.optString(keys.next(), "")
                    }
                }

                Log.d(TAG, "Knock result - success: $success, errCode: $errCode, resolvedHost: $resolvedHost, openTime: $openTime")

                runOnUiThread {
                    if (success && resolvedHost.isNotEmpty()) {
                        val processedUrl = originalUrl.replace(host, resolvedHost)
                        Log.d(TAG, "Loading processed URL: $processedUrl")
                        
                        // Mark as NHP protected page
                        currentPageIsNhp = true
                        
                        // Save mapping: real URL -> original NHP URL (for back/forward navigation)
                        nhpUrlMapping[processedUrl] = originalUrl
                        
                        // Save the original NHP URL to display in address bar
                        currentNhpDisplayUrl = originalUrl
                        
                        nhpStatusText?.text = "NHP Protection Enabled"
                        nhpIndicator?.visibility = View.VISIBLE
                        
                        // Update security icon
                        securityIcon?.setImageResource(R.drawable.ic_shield)
                        securityIcon?.setColorFilter(ContextCompat.getColor(this, R.color.nhp_green))
                        
                        // Update address bar to show original NHP URL
                        urlEditText?.setText(originalUrl)
                        
                        webView?.loadUrl(processedUrl)
                    } else {
                        Log.e(TAG, "Knock failed: $errMsg")
                        currentPageIsNhp = false
                        nhpIndicator?.visibility = View.GONE
                        progressBar?.visibility = View.GONE
                        
                        val errorMessage = if (errMsg.isNotEmpty()) errMsg else "Knock failed (error: $errCode)"
                        Toast.makeText(this, "NHP knock failed: $errorMessage", Toast.LENGTH_SHORT).show()
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "NHP process error", e)
                runOnUiThread {
                    currentPageIsNhp = false
                    nhpIndicator?.visibility = View.GONE
                    progressBar?.visibility = View.GONE
                    Toast.makeText(this, "NHP error: ${e.message}", Toast.LENGTH_SHORT).show()
                }
            }
        }.start()
    }

    private fun updateNavigationButtons() {
        val canGoBack = webView?.canGoBack() == true
        val canGoForward = webView?.canGoForward() == true
        btnBack?.isEnabled = canGoBack
        btnForward?.isEnabled = canGoForward
        btnBack?.alpha = if (canGoBack) 1.0f else 0.3f
        btnForward?.alpha = if (canGoForward) 1.0f else 0.3f
    }

    private fun updateSecurityIndicator(url: String) {
        // Only show NHP indicator if current page was loaded via NHP knock
        if (currentPageIsNhp) {
            securityIcon?.setImageResource(R.drawable.ic_shield)
            securityIcon?.setColorFilter(ContextCompat.getColor(this, R.color.nhp_green))
            nhpIndicator?.visibility = View.VISIBLE
            nhpStatusText?.text = "NHP Protection Enabled"
        } else {
            // Hide NHP indicator for non-NHP pages
            nhpIndicator?.visibility = View.GONE
            
            when {
                url.startsWith("https://") -> {
                    securityIcon?.setImageResource(R.drawable.ic_lock)
                    securityIcon?.setColorFilter(ContextCompat.getColor(this, R.color.nhp_green))
                }
                else -> {
                    securityIcon?.setImageResource(R.drawable.ic_lock_open)
                    securityIcon?.setColorFilter(ContextCompat.getColor(this, R.color.nhp_orange))
                }
            }
        }
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        if (webView?.canGoBack() == true) {
            webView?.goBack()
        } else {
            @Suppress("DEPRECATION")
            super.onBackPressed()
        }
    }

    override fun onDestroy() {
        try {
            webView?.destroy()
        } catch (e: Exception) {
            Log.e(TAG, "Error destroying webview", e)
        }
        try {
            if (nhpInitialized) {
                nhpcore.Nhpcore.cleanup()
            }
        } catch (e: Exception) {
            Log.e(TAG, "Error cleanup nhp", e)
        }
        super.onDestroy()
    }
}
